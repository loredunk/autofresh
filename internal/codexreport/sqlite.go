package codexreport

// Minimal read-only SQLite reader.
//
// This is deliberately tiny: ccoach ships as a single zero-dependency static
// binary, so we cannot pull in a real SQLite driver. We only ever need to do a
// full table scan of the `threads` table in CODEX_HOME/state_*.sqlite, so this
// implements just enough of the SQLite file format for that:
//
//   - the 100-byte database header (page size, text encoding)
//   - table b-tree pages (interior + leaf), with overflow-page following
//   - the record format (varints + serial types)
//
// It does NOT understand the WAL. Callers must check for a sibling "-wal" file
// and fall back to glob-based discovery when one is present, otherwise the read
// could return stale rows. Any parse error is returned so the caller can fall
// back too.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
)

// sqliteDB is an opened, fully-read database file held in memory. state_*.sqlite
// is well under a few MB, so reading it whole keeps page access trivial.
type sqliteDB struct {
	data     []byte
	pageSize int
	encoding uint32 // 1=utf8, 2=utf16le, 3=utf16be
}

func openSQLite(path string) (*sqliteDB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 100 || string(data[:16]) != "SQLite format 3\x00" {
		return nil, errors.New("not a sqlite3 file")
	}
	pageSize := int(binary.BigEndian.Uint16(data[16:18]))
	if pageSize == 1 { // 1 encodes 65536 per the spec
		pageSize = 65536
	}
	if pageSize < 512 || pageSize&(pageSize-1) != 0 {
		return nil, fmt.Errorf("invalid page size %d", pageSize)
	}
	enc := binary.BigEndian.Uint32(data[56:60])
	if enc == 0 {
		enc = 1
	}
	return &sqliteDB{data: data, pageSize: pageSize, encoding: enc}, nil
}

// page returns the 1-based page n. Page 1 carries the 100-byte file header, but
// the b-tree page header still begins at the start of the page; callers offset
// past the file header themselves.
func (db *sqliteDB) page(n int) ([]byte, error) {
	if n < 1 {
		return nil, fmt.Errorf("bad page %d", n)
	}
	start := (n - 1) * db.pageSize
	end := start + db.pageSize
	if end > len(db.data) {
		return nil, fmt.Errorf("page %d out of range", n)
	}
	return db.data[start:end], nil
}

// readVarint decodes a SQLite huffman-ish varint (1-9 bytes, big-endian, high
// bit = continue, the 9th byte contributes all 8 bits). Returns value and width.
func readVarint(b []byte) (int64, int) {
	var result uint64
	for i := 0; i < 8; i++ {
		if i >= len(b) {
			return int64(result), i
		}
		result = (result << 7) | uint64(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return int64(result), i + 1
		}
	}
	if len(b) < 9 {
		return int64(result), len(b)
	}
	result = (result << 8) | uint64(b[8])
	return int64(result), 9
}

// findTableRoot scans the sqlite_master table (rooted at page 1) for a table by
// name and returns its root page number.
func (db *sqliteDB) findTableRoot(name string) (int, error) {
	rows, err := db.scanTable(1, true)
	if err != nil {
		return 0, err
	}
	for _, rec := range rows {
		// sqlite_master columns: type, name, tbl_name, rootpage, sql
		if len(rec) < 4 {
			continue
		}
		if asString(rec[0]) == "table" && asString(rec[1]) == name {
			return int(asInt(rec[3])), nil
		}
	}
	return 0, fmt.Errorf("table %q not found", name)
}

// scanTable walks a table b-tree and returns every leaf record as a slice of
// column values. firstPageHasHeader must be true only for page 1.
func (db *sqliteDB) scanTable(root int, firstPageHasHeader bool) ([][]any, error) {
	var rows [][]any
	err := db.walkPage(root, firstPageHasHeader, &rows, 0)
	return rows, err
}

func (db *sqliteDB) walkPage(pageNum int, hasFileHeader bool, rows *[][]any, depth int) error {
	if depth > 64 {
		return errors.New("b-tree too deep")
	}
	pg, err := db.page(pageNum)
	if err != nil {
		return err
	}
	headerOffset := 0
	if hasFileHeader {
		headerOffset = 100
	}
	h := pg[headerOffset:]
	pageType := h[0]
	numCells := int(binary.BigEndian.Uint16(h[3:5]))

	switch pageType {
	case 0x05: // interior table b-tree
		cellPtrBase := headerOffset + 12
		for i := 0; i < numCells; i++ {
			off := int(binary.BigEndian.Uint16(pg[cellPtrBase+i*2 : cellPtrBase+i*2+2]))
			child := int(binary.BigEndian.Uint32(pg[off : off+4]))
			if err := db.walkPage(child, false, rows, depth+1); err != nil {
				return err
			}
		}
		// rightmost pointer
		right := int(binary.BigEndian.Uint32(h[8:12]))
		return db.walkPage(right, false, rows, depth+1)
	case 0x0d: // leaf table b-tree
		cellPtrBase := headerOffset + 8
		for i := 0; i < numCells; i++ {
			off := int(binary.BigEndian.Uint16(pg[cellPtrBase+i*2 : cellPtrBase+i*2+2]))
			rec, err := db.parseLeafCell(pg, off)
			if err != nil {
				return err
			}
			*rows = append(*rows, rec)
		}
		return nil
	default:
		return fmt.Errorf("unexpected page type 0x%02x", pageType)
	}
}

// parseLeafCell decodes one table-leaf cell into its column values, following
// overflow pages when the payload spills.
func (db *sqliteDB) parseLeafCell(pg []byte, off int) ([]any, error) {
	p := pg[off:]
	payloadLen, n := readVarint(p)
	p = p[n:]
	_, n = readVarint(p) // rowid, unused (id is a real column here)
	p = p[n:]

	usable := db.pageSize // no reserved space assumed
	// Local payload size math per the SQLite spec for table leaves.
	maxLocal := usable - 35
	payload := make([]byte, 0, payloadLen)
	if int(payloadLen) <= maxLocal {
		payload = append(payload, p[:payloadLen]...)
	} else {
		minLocal := ((usable - 12) * 32 / 255) - 23
		k := minLocal + (int(payloadLen)-minLocal)%(usable-4)
		local := minLocal
		if k <= maxLocal {
			local = k
		}
		payload = append(payload, p[:local]...)
		// next 4 bytes: first overflow page number
		nextPage := int(binary.BigEndian.Uint32(p[local : local+4]))
		remaining := int(payloadLen) - local
		for nextPage != 0 && remaining > 0 {
			ovf, err := db.page(nextPage)
			if err != nil {
				return nil, err
			}
			nextPage = int(binary.BigEndian.Uint32(ovf[0:4]))
			chunk := usable - 4
			if chunk > remaining {
				chunk = remaining
			}
			payload = append(payload, ovf[4:4+chunk]...)
			remaining -= chunk
		}
	}
	return db.decodeRecord(payload)
}

// decodeRecord parses a record (header of serial types + body) into Go values.
func (db *sqliteDB) decodeRecord(rec []byte) ([]any, error) {
	if len(rec) == 0 {
		return nil, nil
	}
	hdrLen, n := readVarint(rec)
	if int(hdrLen) > len(rec) {
		return nil, errors.New("record header overruns payload")
	}
	header := rec[n:int(hdrLen)]
	body := rec[int(hdrLen):]
	var out []any
	bodyPos := 0
	for len(header) > 0 {
		serial, sn := readVarint(header)
		header = header[sn:]
		val, size, err := decodeSerial(serial, body[bodyPos:])
		if err != nil {
			return nil, err
		}
		out = append(out, val)
		bodyPos += size
	}
	return out, nil
}

func decodeSerial(serial int64, b []byte) (any, int, error) {
	switch {
	case serial == 0:
		return nil, 0, nil
	case serial == 1:
		return int64(int8(b[0])), 1, nil
	case serial == 2:
		return int64(int16(binary.BigEndian.Uint16(b[:2]))), 2, nil
	case serial == 3:
		v := int64(b[0])<<16 | int64(b[1])<<8 | int64(b[2])
		if v&0x800000 != 0 {
			v -= 1 << 24
		}
		return v, 3, nil
	case serial == 4:
		return int64(int32(binary.BigEndian.Uint32(b[:4]))), 4, nil
	case serial == 5:
		v := int64(0)
		for i := 0; i < 6; i++ {
			v = v<<8 | int64(b[i])
		}
		if v&0x800000000000 != 0 {
			v -= 1 << 48
		}
		return v, 6, nil
	case serial == 6:
		return int64(binary.BigEndian.Uint64(b[:8])), 8, nil
	case serial == 7:
		return math.Float64frombits(binary.BigEndian.Uint64(b[:8])), 8, nil
	case serial == 8:
		return int64(0), 0, nil
	case serial == 9:
		return int64(1), 0, nil
	case serial >= 12 && serial%2 == 0: // BLOB
		size := int((serial - 12) / 2)
		blob := make([]byte, size)
		copy(blob, b[:size])
		return blob, size, nil
	case serial >= 13 && serial%2 == 1: // TEXT
		size := int((serial - 13) / 2)
		return string(b[:size]), size, nil
	default:
		return nil, 0, fmt.Errorf("reserved serial type %d", serial)
	}
}

// helpers to coerce decoded record values.
func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func asInt(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case float64:
		return int64(t)
	default:
		return 0
	}
}
