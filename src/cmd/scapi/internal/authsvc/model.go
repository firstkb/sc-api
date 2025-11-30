package authsvc

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type CleanupExpiredTokensRequest struct{}

type CleanupExpiredRefreshTokensRequest struct{}
