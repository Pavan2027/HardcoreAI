package storage

import "strings"

// BuildFilterSQL constructs a SQL AND-condition fragment from the filter
// fields in SearchOptions. The returned clause uses table aliases that must
// exist in the calling query:
//
//	d → documents
//	c → chunks
//
// Returns ("", nil) when no filter fields are set. Callers must skip the
// WHERE / AND prefix when the clause is empty.
//
// For VectorSearch: append as WHERE <clause> before ORDER BY knn.distance.
// For FTSSearch:    append as AND <clause> after WHERE chunk_fts MATCH ?.
func BuildFilterSQL(opts SearchOptions) (clause string, args []interface{}) {
	var conditions []string

	if opts.ChipFamily != "" {
		conditions = append(conditions, "d.chip_family = ?")
		args = append(args, opts.ChipFamily)
	}
	if opts.ChipModel != "" {
		conditions = append(conditions, "d.chip_model = ?")
		args = append(args, opts.ChipModel)
	}
	if opts.Peripheral != "" {
		conditions = append(conditions, "c.peripheral = ?")
		args = append(args, opts.Peripheral)
	}
	if len(opts.DocTypes) > 0 {
		// Build an IN (?,?,?) placeholder string.
		ph := strings.Repeat("?,", len(opts.DocTypes))
		ph = ph[:len(ph)-1]
		conditions = append(conditions, "d.doc_type IN ("+ph+")")
		for _, dt := range opts.DocTypes {
			args = append(args, dt)
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return strings.Join(conditions, " AND "), args
}