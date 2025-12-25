package server

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/site/bilibili"
)

// handleBilibiliQRGenerate generates a new QR code for Bilibili login
func (s *Server) handleBilibiliQRGenerate(c *gin.Context) {
	auth := bilibili.NewAuth()

	session, err := auth.GenerateQRCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"url":        session.URL,
			"qrcode_key": session.QRCodeKey,
		},
		Message: "QR code generated",
	})
}

// handleBilibiliQRPoll polls the status of QR code login
func (s *Server) handleBilibiliQRPoll(c *gin.Context) {
	qrcodeKey := c.Query("qrcode_key")
	if qrcodeKey == "" {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "qrcode_key is required",
		})
		return
	}

	auth := bilibili.NewAuth()
	status, creds, err := auth.PollQRStatus(qrcodeKey)
	log.Printf("[Bilibili] Poll status: %d (%s), creds: %v, err: %v", status, status.String(), creds != nil, err)

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: err.Error(),
		})
		return
	}

	// Build response
	data := gin.H{
		"status":      int(status),
		"status_text": status.String(),
	}

	// If login confirmed, save credentials and return success
	if status == bilibili.QRConfirmed && creds != nil {
		log.Printf("[Bilibili] Login confirmed! Saving credentials...")
		if err := auth.SaveCredentials(creds); err != nil {
			log.Printf("[Bilibili] Failed to save credentials: %v", err)
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: "failed to save credentials: " + err.Error(),
			})
			return
		}

		// Try to get username
		username, validateErr := auth.ValidateCredentials(creds)
		log.Printf("[Bilibili] Validate result: username=%s, err=%v", username, validateErr)
		if username == "" {
			username = creds.DedeUserID
		}

		data["logged_in"] = true
		data["username"] = username

		// Update server's cached config
		s.cfg = config.LoadOrDefault()
		log.Printf("[Bilibili] Login successful for user: %s", username)
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    data,
		Message: status.String(),
	})
}

// handleBilibiliStatus returns the current Bilibili login status
func (s *Server) handleBilibiliStatus(c *gin.Context) {
	cfg := config.LoadOrDefault()

	if cfg.Bilibili.Cookie == "" {
		c.JSON(http.StatusOK, Response{
			Code: 200,
			Data: gin.H{
				"logged_in": false,
			},
			Message: "not logged in",
		})
		return
	}

	// Parse and validate credentials
	creds := bilibili.ParseCookieString(cfg.Bilibili.Cookie)
	if creds.SESSDATA == "" {
		c.JSON(http.StatusOK, Response{
			Code: 200,
			Data: gin.H{
				"logged_in": false,
			},
			Message: "invalid cookie",
		})
		return
	}

	// Try to validate and get username
	auth := bilibili.NewAuth()
	username, err := auth.ValidateCredentials(creds)
	if err != nil {
		// Cookie exists but validation failed (might be expired)
		c.JSON(http.StatusOK, Response{
			Code: 200,
			Data: gin.H{
				"logged_in": false,
				"error":     err.Error(),
			},
			Message: "cookie expired or invalid",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"logged_in": true,
			"username":  username,
		},
		Message: "logged in",
	})
}
