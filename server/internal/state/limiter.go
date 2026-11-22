package state

// RateCount returns the number of events in bucket after since (Unix seconds).
func (s *Store) RateCount(bucket string, since int64) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM rate_events WHERE bucket = ? AND at > ?`, bucket, since).Scan(&n)
	return n, err
}

func (s *Store) AddRateEvent(bucket string, at int64) error {
	_, err := s.db.Exec(`INSERT INTO rate_events (bucket, at) VALUES (?, ?)`, bucket, at)
	return err
}

func (s *Store) ResetRate(bucket string) error {
	_, err := s.db.Exec(`DELETE FROM rate_events WHERE bucket = ?`, bucket)
	return err
}

func (s *Store) DeleteExpiredRates(before int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM rate_events WHERE at <= ?`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
