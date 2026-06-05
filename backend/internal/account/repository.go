package account

import (
	"context"

	"gorm.io/gorm"
)

type AccountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (ar *AccountRepository) CreateAccount(ctx context.Context, account *Account) error {
	return ar.db.WithContext(ctx).Create(account).Error
}

func (ar *AccountRepository) Rename(ctx context.Context, id uint, newUsername string) error {
	result := ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("username", newUsername)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RenameWithToken 在单个事务中同时更新用户名和 Token。
// 在数据库执行但没有提交。
// 闭包返回 nil → 两个操作一起 COMMIT；返回 error → 两个操作一起 ROLLBACK。
// 事务提交前，其他连接看不到修改（数据库隔离性保证）。
func (ar *AccountRepository) RenameWithToken(ctx context.Context, id uint, newUsername string, token string) error {
	return ar.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Account{}).Where("id = ?", id).Update("username", newUsername)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&Account{}).Where("id = ?", id).Update("token", token).Error; err != nil {
			return err
		}
		return nil
	})
}

func (ar *AccountRepository) ChangePassword(ctx context.Context, id uint, newPassword string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("password", newPassword).Error
}

func (ar *AccountRepository) FindByID(ctx context.Context, id uint) (*Account, error) {
	var account Account
	if err := ar.db.WithContext(ctx).First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// FindByIDs 批量查询账号，返回 map[id]*Account 供调用方按需取用。
func (ar *AccountRepository) FindByIDs(ctx context.Context, ids []uint) (map[uint]*Account, error) {
	result := make(map[uint]*Account, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var accounts []Account
	if err := ar.db.WithContext(ctx).Where("id IN ?", ids).Find(&accounts).Error; err != nil {
		return nil, err
	}
	for i := range accounts {
		result[accounts[i].ID] = &accounts[i]
	}
	return result, nil
}

func (ar *AccountRepository) FindByUsername(ctx context.Context, username string) (*Account, error) {
	var account Account
	if err := ar.db.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (ar *AccountRepository) Login(ctx context.Context, id uint, token, refreshToken string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(map[string]any{"token": token, "refresh_token": refreshToken}).Error
}

func (ar *AccountRepository) Logout(ctx context.Context, id uint) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(map[string]any{"token": "", "refresh_token": ""}).Error
}

func (ar *AccountRepository) UpdateAvatar(ctx context.Context, accountID uint, avatarURL string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).Update("avatar_url", avatarURL).Error
}

func (ar *AccountRepository) UpdateToken(ctx context.Context, id uint, token string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("token", token).Error
}

func (ar *AccountRepository) UpdateFields(ctx context.Context, id uint, updates map[string]interface{}) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(updates).Error
}

func (ar *AccountRepository) FindAll(ctx context.Context) ([]*Account, error) {
	var accounts []*Account
	if err := ar.db.WithContext(ctx).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}
