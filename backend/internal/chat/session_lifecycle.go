package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (s *Service) GetSessionByActionKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ChatSession, error) {
	if s.repo == nil {
		return nil, ErrRepositoryRequired
	}
	key := strings.TrimSpace(actionKey)
	if sellerAccountID <= 0 || key == "" {
		return nil, errors.New("seller_account_id and action_key are required")
	}
	return s.repo.GetSessionByActionKey(ctx, sellerAccountID, key)
}

func (s *Service) resolveSession(ctx context.Context, input AskInput, question string) (*ChatSession, bool, error) {
	if input.SessionID != nil {
		session, err := s.repo.GetSession(ctx, input.SellerAccountID, *input.SessionID)
		if err != nil {
			return nil, false, err
		}
		if session == nil {
			return nil, false, errors.New("chat session was not found")
		}
		if session.Status == ChatSessionStatusArchived {
			reactivated, err := s.repo.ReactivateSession(ctx, input.SellerAccountID, session.ID)
			if err != nil {
				return nil, false, err
			}
			return reactivated, false, nil
		}
		return session, false, nil
	}

	actionKey := strings.TrimSpace(input.ActionKey)
	if actionKey != "" {
		if existing, err := s.repo.GetSessionByActionKey(ctx, input.SellerAccountID, actionKey); err != nil {
			return nil, false, err
		} else if existing != nil {
			if existing.Status == ChatSessionStatusArchived {
				reactivated, err := s.repo.ReactivateSession(ctx, input.SellerAccountID, existing.ID)
				if err != nil {
					return nil, false, err
				}
				return reactivated, false, nil
			}
			return existing, false, nil
		}
		title := s.buildSessionTitle(ctx, input, question)
		created, err := s.repo.CreateSession(ctx, CreateSessionInput{
			SellerAccountID: input.SellerAccountID,
			UserID:          input.UserID,
			Title:           title,
			ActionKey:       &actionKey,
		})
		if err != nil {
			if isUniqueViolation(err) {
				existing, lookupErr := s.repo.GetSessionByActionKey(ctx, input.SellerAccountID, actionKey)
				if lookupErr != nil {
					return nil, false, lookupErr
				}
				if existing == nil {
					return nil, false, err
				}
				return existing, false, nil
			}
			return nil, false, err
		}
		return created, true, nil
	}

	title := BuildChatTitle("", question, "Новый чат")
	created, err := s.repo.CreateSession(ctx, CreateSessionInput{
		SellerAccountID: input.SellerAccountID,
		UserID:          input.UserID,
		Title:           title,
	})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func (s *Service) buildSessionTitle(ctx context.Context, input AskInput, question string) string {
	action := s.resolveActionPromptContext(ctx, input.SellerAccountID, input.ActionKey)
	if action != nil && strings.TrimSpace(action.Title) != "" {
		return BuildChatTitle(action.Title, question, "Новый чат")
	}
	return BuildChatTitle("", question, "Новый чат")
}
