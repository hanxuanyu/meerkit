package store

import (
	"context"
	"encoding/json"
	"time"

	"meerkit/internal/core"
)

func (s *Store) CreateStatusBoardShare(ctx context.Context, share core.StatusBoardShare) error {
	model := &statusBoardShareModel{
		ID: share.ID, Name: share.Name, Token: share.Token,
		MonitorIDsJSON: jsonString(share.MonitorIDs), ItemIDsJSON: jsonString(share.ItemIDs), Active: share.Active, CreatedAt: timestamp(share.CreatedAt),
	}
	_, err := s.orm.NewInsert().Model(model).Exec(ctx)
	return err
}

func (s *Store) ListStatusBoardShares(ctx context.Context) ([]core.StatusBoardShare, error) {
	models := make([]statusBoardShareModel, 0)
	if err := s.orm.NewSelect().Model(&models).OrderExpr("created_at DESC").Scan(ctx); err != nil {
		return nil, err
	}
	shares := make([]core.StatusBoardShare, 0, len(models))
	for index := range models {
		share, err := statusBoardShareToDomain(&models[index])
		if err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, nil
}

func (s *Store) GetStatusBoardShareByToken(ctx context.Context, token string) (core.StatusBoardShare, error) {
	model := new(statusBoardShareModel)
	if err := s.orm.NewSelect().Model(model).Where("token = ?", token).Scan(ctx); err != nil {
		return core.StatusBoardShare{}, err
	}
	return statusBoardShareToDomain(model)
}

func (s *Store) SetStatusBoardShareActive(ctx context.Context, id string, active bool) error {
	_, err := s.orm.NewUpdate().Model((*statusBoardShareModel)(nil)).Set("active = ?", active).Where("id = ?", id).Exec(ctx)
	return err
}

func (s *Store) DeleteStatusBoardShare(ctx context.Context, id string) (bool, error) {
	result, err := s.orm.NewDelete().Model((*statusBoardShareModel)(nil)).Where("id = ? AND active = ?", id, false).Exec(ctx)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func statusBoardShareToDomain(model *statusBoardShareModel) (core.StatusBoardShare, error) {
	share := core.StatusBoardShare{ID: model.ID, Token: model.Token, Name: model.Name, Active: model.Active}
	if err := json.Unmarshal([]byte(model.MonitorIDsJSON), &share.MonitorIDs); err != nil {
		return share, err
	}
	if err := json.Unmarshal([]byte(model.ItemIDsJSON), &share.ItemIDs); err != nil {
		return share, err
	}
	share.CreatedAt, _ = time.Parse(time.RFC3339Nano, model.CreatedAt)
	return share, nil
}
