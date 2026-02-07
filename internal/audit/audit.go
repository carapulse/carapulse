package audit

import (
	"context"
	"errors"
)

type Event struct {
	EventID string
	Payload []byte
}

type Evidence struct {
	EvidenceID  string
	ExecutionID string
	Payload     []byte
}

type Writer interface {
	InsertAuditEvent(ctx context.Context, payload []byte) (string, error)
	InsertEvidence(ctx context.Context, executionID string, payload []byte) (string, error)
}

type Store struct {
	DB Writer
}

func New() *Store {
	return &Store{}
}

func NewWithDB(db Writer) *Store {
	return &Store{DB: db}
}

func (s *Store) AppendEvent(ctx context.Context, ev Event) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.InsertAuditEvent(ctx, ev.Payload)
	return err
}

func (s *Store) StoreEvidence(ctx context.Context, ev Evidence) error {
	if s.DB == nil {
		return nil
	}
	if ev.ExecutionID == "" {
		return errors.New("execution_id required")
	}
	_, err := s.DB.InsertEvidence(ctx, ev.ExecutionID, ev.Payload)
	return err
}
