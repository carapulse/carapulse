package workflows

import (
	"context"
	"errors"
	"testing"
)

type fakeDB struct {
	status string
	err    error
}

func (f fakeDB) GetApprovalStatus(ctx context.Context, planID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.status, nil
}

func TestCheckApprovalApproved(t *testing.T) {
	err := CheckApproval(context.Background(), fakeDB{status: "approved"}, "plan")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCheckApprovalDenied(t *testing.T) {
	err := CheckApproval(context.Background(), fakeDB{status: "denied"}, "plan")
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestCheckApprovalError(t *testing.T) {
	err := CheckApproval(context.Background(), fakeDB{err: errors.New("db")}, "plan")
	if err == nil {
		t.Fatalf("expected err")
	}
}

func TestRequireApprovalNilDB(t *testing.T) {
	if err := RequireApproval(context.Background(), nil, "plan"); err == nil {
		t.Fatalf("expected err")
	}
}

func TestCheckApprovalNilDB(t *testing.T) {
	if err := CheckApproval(context.Background(), nil, "plan"); err == nil {
		t.Fatalf("expected err")
	}
}
