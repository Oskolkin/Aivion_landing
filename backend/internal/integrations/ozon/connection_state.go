package ozon

import (
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// UserConnectionState is the user-facing Ozon connection snapshot (no secrets).
type UserConnectionState struct {
	ConnectionReady      bool
	Status               string
	AuthenticationMode   string
	Missing              []string
	Errors               []ConnectionError
	SellerClientIDMasked string
	SellerCredentialsSet bool
	PerformanceSecretSet bool
	RetentionDeleteAfter *time.Time
}

func ConnectionReady(conn dbgen.OzonConnection) bool {
	if conn.ConnectionStatus == "disconnected_retention" {
		return false
	}
	if conn.DisconnectedAt.Valid {
		return false
	}
	if conn.AuthMode == "PRIVATE_OAUTH" {
		return conn.OauthConnectionState == "CONNECTED" &&
			conn.OauthAccessTokenEncrypted.Valid && conn.OauthRefreshTokenEncrypted.Valid &&
			conn.ConnectionStatus == "connected"
	}
	return conn.SellerApiValid && conn.PerformanceApiValid && conn.ConnectionStatus == "connected"
}

func BuildUserConnectionState(conn dbgen.OzonConnection, service *Service) UserConnectionState {
	state := UserConnectionState{
		ConnectionReady:    ConnectionReady(conn),
		Status:             conn.ConnectionStatus,
		AuthenticationMode: conn.AuthMode,
		Missing:            missingConnectionParts(conn),
		SellerCredentialsSet: strings.TrimSpace(conn.ClientIDEncrypted) != "" &&
			strings.TrimSpace(conn.ApiKeyEncrypted) != "",
		PerformanceSecretSet: conn.PerformanceClientSecretEncrypted.Valid &&
			strings.TrimSpace(conn.PerformanceClientSecretEncrypted.String) != "",
	}
	if conn.AuthMode == "PRIVATE_OAUTH" {
		state.SellerCredentialsSet = false
		state.PerformanceSecretSet = false
	}
	if conn.RetentionDeleteAfter.Valid {
		t := conn.RetentionDeleteAfter.Time
		state.RetentionDeleteAfter = &t
	}
	if service != nil && state.SellerCredentialsSet {
		state.SellerClientIDMasked = service.MaskedClientID(conn)
	}
	if conn.ConnectionErrorMessage.Valid && conn.ConnectionErrorMessage.String != "" {
		code := ""
		if conn.ConnectionErrorCode.Valid {
			code = conn.ConnectionErrorCode.String
		}
		source := "seller_api"
		if !conn.SellerApiValid && conn.PerformanceApiValid {
			source = "seller_api"
		} else if conn.SellerApiValid && !conn.PerformanceApiValid {
			source = "performance_api"
		} else if !conn.SellerApiValid && !conn.PerformanceApiValid {
			source = "seller_api"
		}
		state.Errors = append(state.Errors, ConnectionError{
			Source:  source,
			Code:    code,
			Message: conn.ConnectionErrorMessage.String,
		})
	}
	return state
}

func missingConnectionParts(conn dbgen.OzonConnection) []string {
	if conn.AuthMode == "PRIVATE_OAUTH" {
		if ConnectionReady(conn) {
			return nil
		}
		return []string{"private_oauth"}
	}
	var missing []string
	if !conn.SellerApiValid {
		missing = append(missing, "seller_api")
	}
	if !conn.PerformanceApiValid {
		missing = append(missing, "performance_api")
	}
	return missing
}

func pgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
