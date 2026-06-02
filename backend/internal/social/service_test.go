package social

import (
	"PulseFeed/internal/account"
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMockService 构建完整的 SocialService（含 SocialRepository + AccountRepository）。
// cache 和 socialMQ 设为 nil，测试只覆盖 DB 逻辑。
func newMockService(t *testing.T) (*SocialService, sqlmock.Sqlmock, func()) {
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
	accountRepo := account.NewAccountRepository(gormDB)
	svc := NewSocialService(repo, accountRepo, nil, nil)

	cleanup := func() { db.Close() }
	return svc, mock, cleanup
}

// ============================================================
// validateSocial
// ============================================================

func TestValidateSocial_NilSocial(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	err := svc.validateSocial(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "social is nil") {
		t.Errorf("expected 'social is nil' error, got: %v", err)
	}
}

func TestValidateSocial_ZeroIDs(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	err := svc.validateSocial(context.Background(), &Social{FollowerID: 0, VloggerID: 1})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error for zero follower, got: %v", err)
	}

	err = svc.validateSocial(context.Background(), &Social{FollowerID: 1, VloggerID: 0})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error for zero vlogger, got: %v", err)
	}
}

func TestValidateSocial_SelfFollow(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	err := svc.validateSocial(context.Background(), &Social{FollowerID: 1, VloggerID: 1})
	if err == nil || !strings.Contains(err.Error(), "self") {
		t.Errorf("expected 'self' error, got: %v", err)
	}
}

func TestValidateSocial_AccountNotFound(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// follower 存在
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))

	// vlogger 不存在
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	err := svc.validateSocial(context.Background(), &Social{FollowerID: 1, VloggerID: 999})
	if err == nil {
		t.Fatal("expected error when vlogger not found")
	}
	t.Logf("vlogger not found error (expected): %v", err)
}

func TestValidateSocial_Success(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// follower 存在
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))

	// vlogger 存在
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	err := svc.validateSocial(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected successful validation, got: %v", err)
	}
}

// ============================================================
// Follow
// ============================================================

func TestFollow_Success(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// validateSocial: 两个账号存在
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// FollowIgnoreDuplicate: INSERT 成功
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `socials`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.Follow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected successful follow, got: %v", err)
	}
}

func TestFollow_AlreadyFollowed_Idempotent(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// validateSocial
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// FollowIgnoreDuplicate: 已存在
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `socials`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(gorm.ErrDuplicatedKey)
	mock.ExpectRollback()

	err := svc.Follow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error for duplicate follow (idempotent), got: %v", err)
	}
}

func TestFollow_SelfFollow(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	err := svc.Follow(context.Background(), &Social{FollowerID: 1, VloggerID: 1})
	if err == nil || !strings.Contains(err.Error(), "self") {
		t.Errorf("expected 'self' error, got: %v", err)
	}
}

// ============================================================
// Unfollow
// ============================================================

func TestUnfollow_Success(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// validateSocial
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// DeleteByFollowerAndVlogger: 删除成功
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `socials`").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.Unfollow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected successful unfollow, got: %v", err)
	}
}

func TestUnfollow_NotFollowed_Idempotent(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// validateSocial
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// DeleteByFollowerAndVlogger: 没有匹配行
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `socials`").
		WithArgs(1, 2).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.Unfollow(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error for unfollow when not followed (idempotent), got: %v", err)
	}
}

// ============================================================
// IsFollowed
// ============================================================

func TestService_IsFollowed_True(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// validateSocial
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// IsFollowed: COUNT = 1
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE").
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := svc.IsFollowed(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestService_IsFollowed_False(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// validateSocial
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// IsFollowed: COUNT = 0
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE").
		WithArgs(1, 2).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	ok, err := svc.IsFollowed(context.Background(), &Social{FollowerID: 1, VloggerID: 2})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ok {
		t.Error("expected false")
	}
}

// ============================================================
// CountFollowers / CountFollowing / GetSocialCounts
// ============================================================

func TestCountFollowers(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE vlogger_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	count, err := svc.CountFollowers(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 15 {
		t.Errorf("expected 15, got %d", count)
	}
}

func TestCountFollowers_ZeroID(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	_, err := svc.CountFollowers(context.Background(), 0)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestCountFollowing(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE follower_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := svc.CountFollowing(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestGetSocialCounts(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE vlogger_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `socials` WHERE follower_id").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	counts, err := svc.GetSocialCounts(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if counts.FollowerCount != 100 {
		t.Errorf("expected follower_count=100, got %d", counts.FollowerCount)
	}
	if counts.VloggerCount != 50 {
		t.Errorf("expected vlogger_count=50, got %d", counts.VloggerCount)
	}
}

// ============================================================
// ListFollowers / ListFollowing (service)
// ============================================================

func TestService_ListFollowers(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// accountRepo.FindByID
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "bob", "", "", "", "", ""))

	// repo.ListFollowers 第一步：查 social 关系
	mock.ExpectQuery("SELECT \\* FROM `socials` WHERE vlogger_id").
		WithArgs(2, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "vlogger_id"}).
			AddRow(1, 1, 2))

	// repo.ListFollowers 第二步：查 accounts
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE id IN").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))

	followers, err := svc.ListFollowers(context.Background(), 2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(followers) != 1 {
		t.Errorf("expected 1 follower, got %d", len(followers))
	}
}

func TestService_ListFollowing(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` =").
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(1, "alice", "", "", "", "", ""))

	mock.ExpectQuery("SELECT \\* FROM `socials` WHERE follower_id").
		WithArgs(1, 200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "follower_id", "vlogger_id"}).
			AddRow(1, 1, 2))

	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE id IN").
		WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
			AddRow(2, "charlie", "", "", "", "", ""))

	vloggers, err := svc.ListFollowing(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(vloggers) != 1 {
		t.Errorf("expected 1 vlogger, got %d", len(vloggers))
	}
}

func TestService_ListFollowers_ZeroID(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	_, err := svc.ListFollowers(context.Background(), 0)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}
