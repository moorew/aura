package backup

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/clevercode/sempa/internal/db"
	"github.com/clevercode/sempa/internal/integrations/gmail"
)

// DriveConfigType is the integration_configs key holding the backup Drive token.
const DriveConfigType = "backup_drive"

// DriveTokenResolver returns a DriveTokenFunc that loads the stored Drive token,
// refreshes it when near expiry, persists the refreshed token, and returns the
// access token. Shared by the manual run handler and the daily scheduler.
func DriveTokenResolver(configs *db.IntegrationConfigStore, clientID, clientSecret string) DriveTokenFunc {
	return func(ctx context.Context) (string, error) {
		c, err := configs.Get(ctx, DriveConfigType)
		if err != nil {
			return "", errors.New("Google Drive is not connected")
		}
		var tok gmail.StoredToken
		if err := json.Unmarshal([]byte(c.Config), &tok); err != nil {
			return "", err
		}
		if err := gmail.RefreshAccessToken(ctx, clientID, clientSecret, &tok); err != nil {
			return "", err
		}
		if b, err := json.Marshal(tok); err == nil {
			_, _ = configs.UpdateConfig(ctx, DriveConfigType, string(b))
		}
		return tok.AccessToken, nil
	}
}
