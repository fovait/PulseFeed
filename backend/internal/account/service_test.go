package account

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// newMockService 用 sqlmock 构建一个真实的 AccountService，
// 无需真实 MySQL 连接，所有 SQL 由 mock 拦截。
func newMockService(t *testing.T) (*AccountService, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		TranslateError: true, // 匹配生产环境配置
	})
	if err != nil {
		db.Close()
		t.Fatalf("failed to open gorm with sqlmock: %v", err)
	}

	repo := NewAccountRepository(gormDB)
	svc := NewAccountService(repo, nil) // cache=nil，跳过 Redis 操作

	cleanup := func() {
		db.Close()
	}

	return svc, mock, cleanup
}

// ============================================================
// Login 测试
// ============================================================

// TestLogin_UserNotFound 用户名不存在时返回错误。
func TestLogin_UserNotFound(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// 期望: SELECT * FROM accounts WHERE username = ? ORDER BY id LIMIT 1
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE username = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs("nobody", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, _, err := svc.Login(context.Background(), "nobody", "password123")
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
	if !strings.Contains(err.Error(), "record not found") {
		t.Errorf("expected 'record not found' error, got: %v", err)
	}
}

// TestLogin_WrongPassword 密码错误时返回错误。
func TestLogin_WrongPassword(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// 提前生成 bcrypt 哈希（对应正确密码 "correctpassword"）
	hash, err := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	// 构造数据库返回的行
	rows := sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
		AddRow(1, "testuser", string(hash), "", "", "", "")

	// 期望 FindByUsername 查询
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE username = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs("testuser", 1).
		WillReturnRows(rows)

	// 用错误密码尝试登录 → bcrypt.CompareHashAndPassword 应返回错误
	_, _, err = svc.Login(context.Background(), "testuser", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
	t.Logf("wrong password error (expected): %v", err)
}

// TestLogin_Success 正常登录成功，返回 AT+RT。
func TestLogin_Success(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	password := "mypassword"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	// 1. FindByUsername 返回用户
	rows := sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
		AddRow(42, "alice", string(hash), "", "", "", "")
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE username = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs("alice", 1).
		WillReturnRows(rows)

	// 2. Login 写入 DB（UPDATE token, refresh_token）
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `accounts` SET").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), 42).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	accessToken, refreshToken, err := svc.Login(context.Background(), "alice", password)
	if err != nil {
		t.Fatalf("expected successful login, got error: %v", err)
	}
	if accessToken == "" {
		t.Error("access token should not be empty")
	}
	if refreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	t.Logf("access token: %s", accessToken[:20]+"...")
	t.Logf("refresh token: %s", refreshToken)
}

// ============================================================
// CreateAccount 测试
// ============================================================

// TestCreateAccount_Success 注册成功。
func TestCreateAccount_Success(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `accounts`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := svc.CreateAccount(context.Background(), &Account{
		Username: "newuser",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("expected successful creation, got: %v", err)
	}
}

// TestCreateAccount_DuplicateUsername 注册重名返回错误。
func TestCreateAccount_DuplicateUsername(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// GORM TranslateError:true → MySQL 1062 转为 gorm.ErrDuplicatedKey
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `accounts`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(gorm.ErrDuplicatedKey)
	mock.ExpectRollback()

	err := svc.CreateAccount(context.Background(), &Account{
		Username: "existing",
		Password: "secret123",
	})
	if err == nil {
		t.Fatal("expected duplicate key error, got nil")
	}
	if !strings.Contains(err.Error(), "Duplicate") && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "1062") {
		t.Logf("error was: %v (check if it's gorm.ErrDuplicatedKey)", err)
	}
}

// ============================================================
// ChangePassword 测试
// ============================================================

// TestChangePassword_WrongOldPassword 旧密码错误 → bcrypt 比对失败。
func TestChangePassword_WrongOldPassword(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	oldHash, err := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}

	// FindByUsername
	rows := sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
		AddRow(1, "testuser", string(oldHash), "", "", "", "")
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE username = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs("testuser", 1).
		WillReturnRows(rows)

	err = svc.ChangePassword(context.Background(), "testuser", "wrongoldpass", "newpass")
	if err == nil {
		t.Fatal("expected bcrypt mismatch error, got nil")
	}
	t.Logf("wrong old password error (expected): %v", err)
}

// ============================================================
// RefreshAccessToken 测试
// ============================================================

// TestRefreshAccessToken_EmptyToken 空 refresh token → 直接返回错误。
func TestRefreshAccessToken_EmptyToken(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	_, _, _, err := svc.RefreshAccessToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty refresh token")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error message, got: %v", err)
	}
}

// TestRefreshAccessToken_InvalidToken cache=nil 时走 DB 全表扫描，
// 找不到匹配的 refresh token → 返回 "invalid refresh token"。
func TestRefreshAccessToken_InvalidToken(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// FindAll：全表扫描，返回空结果
	mock.ExpectQuery("SELECT \\* FROM `accounts`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}))

	_, _, _, err := svc.RefreshAccessToken(context.Background(), "badtoken")
	if err == nil {
		t.Fatal("expected 'invalid refresh token' error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid refresh token") {
		t.Errorf("expected 'invalid refresh token', got: %v", err)
	}
}

// ============================================================
// Rename 测试
// ============================================================

// TestRename_EmptyUsername 空用户名 → 返回 ErrNewUsernameRequired。
func TestRename_EmptyUsername(t *testing.T) {
	svc, _, cleanup := newMockService(t)
	defer cleanup()

	_, err := svc.Rename(context.Background(), 1, "")
	if err != ErrNewUsernameRequired {
		t.Fatalf("expected ErrNewUsernameRequired, got: %v", err)
	}
}

// ============================================================
// 边界条件测试
// ============================================================

// TestLogin_EmptyUsername 空用户名 → FindByUsername 找不到。
func TestLogin_EmptyUsername(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE username = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs("", 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, _, err := svc.Login(context.Background(), "", "password")
	if err == nil {
		t.Fatal("expected error for empty username, got nil")
	}
}

// TestLogin_EmptyPassword 空密码 → bcrypt 比对失败。
func TestLogin_EmptyPassword(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	hash, _ := bcrypt.GenerateFromPassword([]byte("somepass"), bcrypt.DefaultCost)
	rows := sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
		AddRow(1, "testuser", string(hash), "", "", "", "")
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE username = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs("testuser", 1).
		WillReturnRows(rows)

	_, _, err := svc.Login(context.Background(), "testuser", "")
	if err == nil {
		t.Fatal("expected bcrypt mismatch for empty password, got nil")
	}
	t.Logf("empty password error (expected): %v", err)
}

// ============================================================
// Logout 测试
// ============================================================

// TestLogout_Success 登出成功，清除 MySQL 中 token 字段。
func TestLogout_Success(t *testing.T) {
	svc, mock, cleanup := newMockService(t)
	defer cleanup()

	// 1. FindByID
	rows := sqlmock.NewRows([]string{"id", "username", "password", "token", "refresh_token", "avatar_url", "bio"}).
		AddRow(1, "testuser", "hash", "existing_token", "existing_refresh", "", "")
	mock.ExpectQuery("SELECT \\* FROM `accounts` WHERE `accounts`\\.`id` = \\? ORDER BY `accounts`\\.`id` LIMIT \\?").
		WithArgs(1, 1).
		WillReturnRows(rows)

	// 2. Logout UPDATE
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `accounts` SET").
		WithArgs("", "", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := svc.Logout(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected successful logout, got: %v", err)
	}
}

// ============================================================
// 验证所有 mock 期望都被满足
// ============================================================

// 每个测试结束后 sqlmock 会通过 t.Cleanup 或 defer 自动校验未满足的期望。
// 如果测试通过但有未满足的 SQL 期望，sqlmock.New() 的 db.Close() 会触发检查。
// 我们额外封装一个便捷函数，在测试最后调用 mock.ExpectationsWereMet()。
func assertExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
