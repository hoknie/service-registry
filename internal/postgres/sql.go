package postgres

func RFC3339(col string) string {
	return `to_char(` + col + ` AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`
}
