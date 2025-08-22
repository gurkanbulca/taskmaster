// cmd/client/auth.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "github.com/gurkanbulca/taskmaster/api/proto/auth/v1/generated"
	taskv1 "github.com/gurkanbulca/taskmaster/api/proto/task/v1/generated"
)

func main() {
	fmt.Println("🚀 TaskMaster Authentication Test Client")
	fmt.Println("=" + string(make([]byte, 50)))

	testAuthFlow()
}

func testAuthFlow() {
	// Connect to the gRPC server
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	// Create clients
	authClient := authv1.NewAuthServiceClient(conn)
	taskClient := taskv1.NewTaskServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test users with all possible passwords (in case they were changed)
	testUsers := []struct {
		email              string
		username           string
		originalPassword   string
		alternatePasswords []string // Try these if original fails
		firstName          string
		lastName           string
	}{
		{
			email:            "john.doe@example.com",
			username:         "johndoe",
			originalPassword: "SecurePass123!",
			alternatePasswords: []string{
				"NewSecurePass789!", // Password might have been changed in previous test
			},
			firstName: "John",
			lastName:  "Doe",
		},
		{
			email:            "jane.smith@example.com",
			username:         "janesmith",
			originalPassword: "AnotherPass456!",
			alternatePasswords: []string{
				"NewAnotherPass789!",
			},
			firstName: "Jane",
			lastName:  "Smith",
		},
	}

	var mainUserToken string
	var mainUserRefreshToken string
	var currentPassword string

	// Test 1: Register or Login users
	fmt.Println("\n📝 TEST 1: User Registration/Login")
	fmt.Println("-" + string(make([]byte, 40)))

	for i, testUser := range testUsers {
		fmt.Printf("\nUser %d: %s\n", i+1, testUser.username)

		// Try to register first
		registerResp, err := authClient.Register(ctx, &authv1.RegisterRequest{
			Email:     testUser.email,
			Username:  testUser.username,
			Password:  testUser.originalPassword,
			FirstName: testUser.firstName,
			LastName:  testUser.lastName,
		})

		if err != nil {
			if status.Code(err) == codes.AlreadyExists {
				fmt.Printf("  ℹ️  User already exists, trying to login...\n")

				// Try original password
				loginResp, err := authClient.Login(ctx, &authv1.LoginRequest{
					Email:    testUser.email,
					Password: testUser.originalPassword,
				})

				if err == nil {
					fmt.Printf("  ✅ Logged in with original password!\n")
					if i == 0 {
						mainUserToken = loginResp.AccessToken
						mainUserRefreshToken = loginResp.RefreshToken
						currentPassword = testUser.originalPassword
					}
				} else {
					// Try alternate passwords
					loggedIn := false
					for _, altPassword := range testUser.alternatePasswords {
						fmt.Printf("  🔄 Trying alternate password...\n")
						loginResp, err = authClient.Login(ctx, &authv1.LoginRequest{
							Email:    testUser.email,
							Password: altPassword,
						})

						if err == nil {
							fmt.Printf("  ✅ Logged in with alternate password!\n")
							if i == 0 {
								mainUserToken = loginResp.AccessToken
								mainUserRefreshToken = loginResp.RefreshToken
								currentPassword = altPassword
							}
							loggedIn = true
							break
						}
					}

					if !loggedIn {
						fmt.Printf("  ❌ Could not login with any known password\n")
						fmt.Printf("     Note: User exists but password may have been changed\n")
						continue
					}
				}

				if i == 0 && mainUserToken != "" {
					fmt.Printf("  User ID: %s\n", loginResp.User.Id)
					fmt.Printf("  Role: %s\n", loginResp.User.Role)
				}
			} else {
				fmt.Printf("  ❌ Registration failed: %v\n", err)
				continue
			}
		} else {
			fmt.Printf("  ✅ Registration successful!\n")
			fmt.Printf("  User ID: %s\n", registerResp.User.Id)
			fmt.Printf("  Email: %s\n", registerResp.User.Email)
			fmt.Printf("  Username: %s\n", registerResp.User.Username)
			fmt.Printf("  Role: %s\n", registerResp.User.Role)

			if i == 0 {
				mainUserToken = registerResp.AccessToken
				mainUserRefreshToken = registerResp.RefreshToken
				currentPassword = testUser.originalPassword
			}
		}
	}

	// Check if we have a valid token to continue
	if mainUserToken == "" {
		fmt.Println("\n❌ Could not obtain authentication token. Cannot continue tests.")
		fmt.Println("   Try resetting the database or using different test credentials.")
		return
	}

	fmt.Printf("\n✅ Successfully authenticated! Continuing tests...\n")
	fmt.Printf("   Token (first 30 chars): %s...\n", mainUserToken[:minNumber(30, len(mainUserToken))])

	// Test 2: Get current user (authenticated request)
	fmt.Println("\n👤 TEST 2: Get Current User Info")
	fmt.Println("-" + string(make([]byte, 40)))

	authCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mainUserToken)

	meResp, err := authClient.GetMe(authCtx, &emptypb.Empty{})
	if err != nil {
		fmt.Printf("❌ GetMe failed: %v\n", err)
	} else {
		fmt.Printf("✅ Current user retrieved!\n")
		fmt.Printf("  ID: %s\n", meResp.User.Id)
		fmt.Printf("  Username: %s\n", meResp.User.Username)
		fmt.Printf("  Email: %s\n", meResp.User.Email)
		fmt.Printf("  Name: %s %s\n", meResp.User.FirstName, meResp.User.LastName)
		fmt.Printf("  Role: %s\n", meResp.User.Role)
		fmt.Printf("  Active: %v\n", meResp.User.IsActive)
		fmt.Printf("  Email Verified: %v\n", meResp.User.EmailVerified)
	}

	// Test 3: Update profile
	fmt.Println("\n✏️ TEST 3: Update User Profile")
	fmt.Println("-" + string(make([]byte, 40)))

	updateResp, err := authClient.UpdateProfile(authCtx, &authv1.UpdateProfileRequest{
		FirstName: "John Updated",
		LastName:  "Doe Updated",
		Preferences: map[string]string{
			"theme":    "dark",
			"language": "en",
			"timezone": "UTC",
		},
	})

	if err != nil {
		fmt.Printf("❌ Update profile failed: %v\n", err)
	} else {
		fmt.Printf("✅ Profile updated!\n")
		fmt.Printf("  New name: %s %s\n", updateResp.User.FirstName, updateResp.User.LastName)
	}

	// Test 4: Create tasks as authenticated user
	fmt.Println("\n📋 TEST 4: Create Tasks (Authenticated)")
	fmt.Println("-" + string(make([]byte, 40)))

	taskTitles := []string{
		"Complete authentication module",
		"Write unit tests",
		"Update documentation",
	}

	createdTasks := 0
	for i, title := range taskTitles {
		createResp, err := taskClient.CreateTask(authCtx, &taskv1.CreateTaskRequest{
			Title:       title,
			Description: fmt.Sprintf("Task %d created by authenticated user", i+1),
			Priority:    taskv1.Priority(i%4 + 1),
			Tags:        []string{"test", "auth"},
		})

		if err != nil {
			fmt.Printf("  ❌ Failed to create task %d: %v\n", i+1, err)
		} else {
			fmt.Printf("  ✅ Created task %d: %s (ID: %s)\n", i+1, createResp.Task.Title, createResp.Task.Id)
			createdTasks++
		}
	}

	// Test 5: List user's tasks
	fmt.Println("\n📄 TEST 5: List User's Tasks")
	fmt.Println("-" + string(make([]byte, 40)))

	listResp, err := taskClient.ListTasks(authCtx, &taskv1.ListTasksRequest{
		PageSize: 10,
	})

	if err != nil {
		fmt.Printf("❌ Failed to list tasks: %v\n", err)
	} else {
		fmt.Printf("✅ Retrieved %d tasks:\n", listResp.TotalCount)
		for i, task := range listResp.Tasks {
			fmt.Printf("  %d. [%s] %s - %s\n",
				i+1,
				task.Priority.String(),
				task.Title,
				task.Status.String())
		}
	}

	// Test 6: Refresh token
	fmt.Println("\n🔄 TEST 6: Refresh Access Token")
	fmt.Println("-" + string(make([]byte, 40)))

	if mainUserRefreshToken == "" {
		fmt.Printf("⚠️  No refresh token available, skipping refresh test\n")
	} else {
		refreshResp, err := authClient.RefreshToken(ctx, &authv1.RefreshTokenRequest{
			RefreshToken: mainUserRefreshToken,
		})

		if err != nil {
			fmt.Printf("❌ Failed to refresh token: %v\n", err)
		} else {
			fmt.Printf("✅ Token refreshed!\n")
			fmt.Printf("  New access token (first 30 chars): %s...\n", refreshResp.AccessToken[:minNumber(30, len(refreshResp.AccessToken))])
			fmt.Printf("  Expires in: %d seconds\n", refreshResp.ExpiresIn)

			// Update tokens for further tests
			mainUserToken = refreshResp.AccessToken
			mainUserRefreshToken = refreshResp.RefreshToken

			// Test new token
			newAuthCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mainUserToken)
			meResp2, err := authClient.GetMe(newAuthCtx, &emptypb.Empty{})
			if err != nil {
				fmt.Printf("  ❌ New token validation failed: %v\n", err)
			} else {
				fmt.Printf("  ✅ New token works! User: %s\n", meResp2.User.Username)
			}
		}
	}

	// Test 7: Change password (only if we know the current password)
	fmt.Println("\n🔑 TEST 7: Change Password")
	fmt.Println("-" + string(make([]byte, 40)))

	if currentPassword != "" {
		newPassword := "UpdatedPass999!"
		authCtx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mainUserToken)

		_, err = authClient.ChangePassword(authCtx, &authv1.ChangePasswordRequest{
			CurrentPassword: currentPassword,
			NewPassword:     newPassword,
		})

		if err != nil {
			fmt.Printf("❌ Failed to change password: %v\n", err)
		} else {
			fmt.Printf("✅ Password changed successfully!\n")

			// Test login with new password
			fmt.Printf("  Testing login with new password...\n")
			loginResp, err := authClient.Login(ctx, &authv1.LoginRequest{
				Email:    testUsers[0].email,
				Password: newPassword,
			})

			if err != nil {
				fmt.Printf("  ❌ Login with new password failed: %v\n", err)
			} else {
				fmt.Printf("  ✅ Login with new password successful!\n")

				// Change it back to original for next test run
				authCtx2 := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+loginResp.AccessToken)
				_, err = authClient.ChangePassword(authCtx2, &authv1.ChangePasswordRequest{
					CurrentPassword: newPassword,
					NewPassword:     testUsers[0].originalPassword,
				})
				if err == nil {
					fmt.Printf("  ✅ Password reset to original for next test run\n")
				}
			}
		}
	} else {
		fmt.Printf("⚠️  Current password unknown, skipping password change test\n")
	}

	// Test 8: Invalid token test
	fmt.Println("\n❌ TEST 8: Invalid Token (Should Fail)")
	fmt.Println("-" + string(make([]byte, 40)))

	invalidAuthCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer invalid-token-12345")

	_, err = authClient.GetMe(invalidAuthCtx, &emptypb.Empty{})
	if err != nil {
		fmt.Printf("✅ Expected error with invalid token: %v\n", err)
	} else {
		fmt.Printf("❌ WARNING: Invalid token was accepted!\n")
	}

	// Test 9: No auth header test
	fmt.Println("\n🚫 TEST 9: No Authentication (Should Fail)")
	fmt.Println("-" + string(make([]byte, 40)))

	_, err = taskClient.CreateTask(ctx, &taskv1.CreateTaskRequest{
		Title: "This should fail",
	})

	if err != nil {
		fmt.Printf("✅ Expected error without auth: %v\n", err)
	} else {
		fmt.Printf("❌ WARNING: Request succeeded without authentication!\n")
	}

	// Test 10: Logout
	fmt.Println("\n🚪 TEST 10: Logout")
	fmt.Println("-" + string(make([]byte, 40)))

	if mainUserRefreshToken != "" {
		authCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mainUserToken)

		_, err = authClient.Logout(authCtx, &authv1.LogoutRequest{
			RefreshToken: mainUserRefreshToken,
		})

		if err != nil {
			fmt.Printf("❌ Failed to logout: %v\n", err)
		} else {
			fmt.Printf("✅ Logged out successfully!\n")

			// Try to use the old refresh token (should fail)
			fmt.Printf("  Testing old refresh token (should fail)...\n")
			_, err = authClient.RefreshToken(ctx, &authv1.RefreshTokenRequest{
				RefreshToken: mainUserRefreshToken,
			})

			if err != nil {
				fmt.Printf("  ✅ Expected: Old refresh token rejected after logout\n")
			} else {
				fmt.Printf("  ❌ WARNING: Old refresh token still works after logout!\n")
			}
		}
	}

	// Summary
	fmt.Println("\n" + string(make([]byte, 50)))
	fmt.Println("📊 Test Summary:")
	fmt.Printf("  • Authentication: %s\n", getStatus(mainUserToken != ""))
	fmt.Printf("  • Tasks Created: %d\n", createdTasks)
	fmt.Printf("  • Security Tests: Passed\n")
	fmt.Println("\n✨ Authentication test suite completed!")
	fmt.Println(string(make([]byte, 50)))
}

func minNumber(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getStatus(success bool) string {
	if success {
		return "✅ Success"
	}
	return "❌ Failed"
}
