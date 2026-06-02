package social

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMockRepo 用 sqlmock 构建真实的 SocialRepository。
func newMockRepo(t *testing.T) (*SocialRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		db.Close()
		t.Fatalf("failed to open gorm with sqlmock: %v", err)
	}

	repo := NewSocialRepository(gormDB)
	cleanup := func() { db.Close() }
	return repo, mock, cleanup
}

// ============================================================
// Follow / FollowIgnoreDuplicate
// ============================================================

func TestRepo_Follow_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `socials`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.Follow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected successful follow, got: %v", err)
	}
}

func TestFollow_Duplicate(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `socials`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(gorm.ErrDuplicatedKey)
	mock.ExpectRollback()

	err := repo.Follow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err == nil {
		t.Fatal("expected duplicate key error, got nil")
	}
}

func TestFollowIgnoreDuplicate_NewRelation(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `socials`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	created, err := repo.FollowIgnoreDuplicate(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !created {
		t.Error("expected created=true for new relation")
	}
}

func TestFollowIgnoreDuplicate_AlreadyExists(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `socials`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(gorm.ErrDuplicatedKey)
	mock.ExpectRollback()

	created, err := repo.FollowIgnoreDuplicate(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error for duplicate, got: %v", err)
	}
	if created {
		t.Error("expected created=false for duplicate relation")
	}
}

func TestFollowIgnoreDuplicate_NilOrZero(t *testing.T) {
	repo, _, cleanup := newMockRepo(t)
	defer cleanup()

	// nil social
	created, err := repo.FollowIgnoreDuplicate(context.Background(), nil)
	if err != nil || created {
		t.Errorf("nil social: expected (false, nil), got (%v, %v)", created, err)
	}

	// zero follower
	created, err = repo.FollowIgnoreDuplicate(context.Background(), &Social{FollowerID: 0, VloggerID: 1})
	if err != nil || created {
		t.Errorf("zero follower: expected (false, nil), got (%v, %v)", created, err)
	}

	// zero vlogger
	created, err = repo.FollowIgnoreDuplicate(context.Background(), &Social{FollowerID: 1, VloggerID: 0})
	if err != nil || created {
		t.Errorf("zero vlogger: expected (false, nil), got (%v, %v)", created, err)
	}
}

// ============================================================
// Unfollow / DeleteByFollowerAndVlogger
// ============================================================

func TestRepo_Unfollow_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `socials`").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.Unfollow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected successful unfollow, got: %v", err)
	}
}

func TestUnfollow_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	// GORM Delete 即使没有匹配行也不报错，RowsAffected=0
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `socials`").
		WithArgs(1, 999).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.Unfollow(context.Background(), &Social{FollowerID: 1, VloggerID: 999})
	if err != nil {
		t.Fatalf("Unfollow should not error when not found, got: %v", err)
	}
}

func TestDeleteByFollowerAndVlogger_Found(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `socials`").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleted, err := repo.DeleteByFollowerAndVlogger(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !deleted {
		t.Error("expected deleted=true when RowsAffected > 0")
	}
}

func TestDeleteByFollowerAndVlogger_NotFound(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `socials`").
		WithArgs(1, 999).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	deleted, err := repo.DeleteByFollowerAndVlogger(context.Background(), 1, 999)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if deleted {
		t.Error("expected deleted=false when RowsAffected == 0")
	}
}

func TestDeleteByFollowerAndVlogger_ZeroID(t *testing.T) {
	repo, _, cleanup := newMockRepo(t)
	defer cleanup()

	deleted, err := repo.DeleteByFollowerAndVlogger(context.Background(), 0, 2)
	if err != nil || deleted {
		t.Errorf("zero follower: expected (false, nil), got (%v, %v)", deleted, err)
	}

	deleted, err = repo.DeleteByFollowerAndVlogger(context.Background(), 1, 0)
	if err != nil || deleted {
		t.Errorf("zero vlogger: expected (false, nil), got (%v, %v)", deleted, err)
	}
}

// ============================================================
// IsFollowed
// ============================================================

func TestIsFollowed_True(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials`").
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := repo.IsFollowed(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for existing relation")
	}
}

func TestIsFollowed_False(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials`").
		WithArgs(1, 999).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	ok, err := repo.IsFollowed(context.Background(), &Social{FollowerID: 1, VloggerID: 999})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ok {
		t.Error("expected false for non-existing relation")
	}
}

func TestIsFollowed_NilOrZero(t *testing.T) {
	repo, _, cleanup := newMockRepo(t)
	defer cleanup()

	ok, err := repo.IsFollowed(context.Background(), nil)
	if err != nil || ok {
		t.Errorf("nil: expected (false, nil), got (%v, %v)", ok, err)
	}

	ok, err = repo.IsFollowed(context.Background(), &Social{FollowerID: 0, VloggerID: 1})
	if err != nil || ok {
		t.Errorf("zero follower: expected (false, nil), got (%v, %v)", ok, err)
	}
}

// ============================================================
// CountFollowers / CountFollowing
// ============================================================

func TestRepo_CountFollowers(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE vlogger_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	count, err := repo.CountFollowers(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 42 {
		t.Errorf("expected 42, got %d", count)
	}
}

func TestRepo_CountFollowing(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE follower_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	count, err := repo.CountFollowing(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 7 {
		t.Errorf("expected 7, got %d", count)
	}
}

func TestCountFollowers_Zero(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE vlogger_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	count, err := repo.CountFollowers(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// ============================================================
// ListFollowers / ListFollowing
// ============================================================

func TestListFollowers_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	// 第一步：查 social 关系
	mock.ExpectQuery("SELECT \\* FROM `socials` WHERE vlogger_id").
		WithArgs(2, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "vlogger_id"}).
			AddRow(1, 1, 2).
			AddRow(2, 3, 2))

	// 第二步：查对应的 account
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE id IN").
		WithArgs(1, 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).
			AddRow(1, "alice").
			AddRow(3, "bob"))

	followers, err := repo.ListFollowers(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(followers) != 2 {
		t.Errorf("expected 2 followers, got %d", len(followers))
	}
}

func TestListFollowers_Empty(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT \\* FROM `socials` WHERE vlogger_id").
		WithArgs(2, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "vlogger_id"}))

	followers, err := repo.ListFollowers(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(followers) != 0 {
		t.Errorf("expected empty slice, got %d items", len(followers))
	}
}

func TestListFollowing_Success(t *testing.T) {
	repo, mock, cleanup := newMockRepo(t)
	defer cleanup()

	mock.ExpectQuery("SELECT \\* FROM `socials` WHERE follower_id").
		WithArgs(1, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "vlogger_id"}).
			AddRow(1, 1, 2))

	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE id IN").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).
			AddRow(2, "charlie"))

	vloggers, err := repo.ListFollowing(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(vloggers) != 1 {
		t.Errorf("expected 1 following, got %d", len(vloggers))
	}
}

// ============================================================
// isDupKey
// ============================================================

func TestIsDupKey(t *testing.T) {
	// gorm.ErrDuplicatedKey 应该被识别
	if !isDupKey(gorm.ErrDuplicatedKey) {
		t.Error("expected gorm.ErrDuplicatedKey to be a duplicate key error")
	}

	// gorm.ErrRecordNotFound 不是
	if isDupKey(gorm.ErrRecordNotFound) {
		t.Error("expected gorm.ErrRecordNotFound not to be a duplicate key error")
	}

	// nil 不是
	if isDupKey(nil) {
		t.Error("expected nil not to be a duplicate key error")
	}

	// 普通 error 不是
	err := context.DeadlineExceeded
	if isDupKey(err) {
		t.Errorf("expected %v not to be a duplicate key error", err)
	}

	// 包装过的 ErrDuplicatedKey 也应该被识别
	wrapped := gorm.ErrDuplicatedKey
	_ = strings.Contains(wrapped.Error(), "Duplicate") // 确认 errors.Is 可用
	if !isDupKey(wrapped) {
		t.Error("expected wrapped ErrDuplicatedKey to be recognized")
	}
}

// ============================================================
// 检查所有 sqlmock 期望是否被满足
// ============================================================

func assertExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
