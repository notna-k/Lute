package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/api/option"

	"github.com/lute/api/models"
	"github.com/lute/api/repository"
)

var firebaseApp *firebase.App
var firebaseAuth *auth.Client

// InitFirebase initializes Firebase Admin SDK
func InitFirebase(projectID string) error {
	if firebaseApp != nil {
		return nil // Already initialized
	}

	ctx := context.Background()
	var opt option.ClientOption

	// Try to use credentials from environment variable
	credsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
	if credsJSON != "" {
		opt = option.WithCredentialsJSON([]byte(credsJSON))
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: projectID,
	}, opt)
	if err != nil {
		return fmt.Errorf("error initializing Firebase app: %v", err)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("error getting Auth client: %v", err)
	}

	firebaseApp = app
	firebaseAuth = authClient
	return nil
}

// verifyFirebaseToken verifies a Firebase ID token and returns the user ID and email
func verifyFirebaseToken(ctx context.Context, idToken string) (string, string, error) {
	if firebaseAuth == nil {
		return "", "", fmt.Errorf("Firebase not initialized")
	}

	token, err := firebaseAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return "", "", fmt.Errorf("error verifying ID token: %v", err)
	}

	// Extract user ID (UID) from token
	uid := token.UID

	// Extract email from token claims
	email, _ := token.Claims["email"].(string)

	return uid, email, nil
}

// AuthMiddleware validates Firebase JWT tokens and sets user context
func AuthMiddleware(userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token required"})
			c.Abort()
			return
		}

		// Verify Firebase token and extract claims
		ctx := c.Request.Context()
		firebaseUID, email, err := verifyFirebaseToken(ctx, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// Look up or create user in MongoDB
		user, err := userRepo.GetByFirebaseUID(ctx, firebaseUID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Create new user if doesn't exist
				user = &models.User{
					FirebaseUID: firebaseUID,
					Email:       email,
				}
				if err := userRepo.Create(ctx, user); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
					c.Abort()
					return
				}
				// Fetch the created user to get the ID
				user, err = userRepo.GetByFirebaseUID(ctx, firebaseUID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created user"})
					c.Abort()
					return
				}
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lookup user"})
				c.Abort()
				return
			}
		}

		// Set user ID in context (MongoDB ObjectID as hex string)
		c.Set("user_id", user.ID.Hex())
		c.Set("firebase_uid", firebaseUID)
		c.Set("token", token)

		c.Next()
	}
}

// OptionalAuthMiddleware allows requests with or without auth
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				c.Set("token", parts[1])
				// TODO: Verify and extract user ID from token
			}
		}
		c.Next()
	}
}
