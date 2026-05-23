package repos

import (
	"context"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	g *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{g: db}
}

func (r *UserRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return mapErr(r.q(ctx).Create(user).Error)
}

func (r *UserRepository) GetByID(ctx context.Context, uid id.ID) (*models.User, error) {
	var u models.User
	if err := r.q(ctx).Where("id = ?", uid.Hex()).First(&u).Error; err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (r *UserRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*models.User, error) {
	var u models.User
	if err := r.q(ctx).Where("firebase_uid = ?", firebaseUID).First(&u).Error; err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := r.q(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, uid id.ID, user *models.User) error {
	user.ID = uid
	return mapErr(r.q(ctx).Save(user).Error)
}

func (r *UserRepository) Delete(ctx context.Context, uid id.ID) error {
	return mapErr(r.q(ctx).Where("id = ?", uid.Hex()).Delete(&models.User{}).Error)
}
