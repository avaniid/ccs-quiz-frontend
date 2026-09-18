package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	login "ccs.quizportal/Login"
	quiz "ccs.quizportal/Quiz"
	registerationform "ccs.quizportal/Registeration_form"
	admin "ccs.quizportal/admin_portal"
	"ccs.quizportal/db"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system ENV")
	}
	gothic.Store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))

	if err := db.Init(); err != nil {
		log.Fatalf("Failed to initialize MongoDB: %v", err)
	}

	// quiz.CalcScore()

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	clientCallbackURL := os.Getenv("CLIENT_CALLBACK_URL")

	if clientID == "" || clientSecret == "" || clientCallbackURL == "" {
		log.Fatal("env variables for oauth missing")
	}

	goth.UseProviders(
		google.New(clientID, clientSecret, clientCallbackURL),
	)

	// Allowed browser origins: the local Vite/CRA dev servers plus whatever the
	// deployed frontend is. FRONTEND_URL may hold several comma separated origins;
	// empty entries are dropped so an unset env var can't whitelist "".
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:5173"}
	for _, origin := range strings.Split(os.Getenv("FRONTEND_URL"), ",") {
		if origin = strings.TrimRight(strings.TrimSpace(origin), "/"); origin != "" {
			allowedOrigins = append(allowedOrigins, origin)
		}
	}

	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	router.LoadHTMLGlob("admin_portal/templates/*")
	router.GET("/health", checkHealth)
	router.GET("/verify", login.Verify_token)
	router.GET("/checkRegistered", registerationform.Check_Registered)

	router.GET("/auth/google", login.HandleAuth)

	router.GET("/google_callback", login.HandleAuthCallback)

	router.GET("/logout", login.Logout)
	router.GET("/admin", func(c *gin.Context) {
		auth, err := c.Cookie("admin_auth")
		if err != nil || auth != "main" && auth != "view" {
			c.Redirect(http.StatusFound, "/admin/login")
			return
		}
		if auth == "main" {
			c.Redirect(http.StatusFound, "/admin/home")
			return
		}
		if auth == "view" {
			c.Redirect(http.StatusFound, "/admin/view")
			return
		}

	})
	router.GET("/admin/login", admin.ShowLogin)
	router.POST("/admin/login", admin.HandleLogin)
	router.Static("/static", "./admin_portal/static")
	router.GET("/quiz/shifts", quiz.GetAppConfig)

	authorized_admin := router.Group("/admin", admin.MainAdminMiddleware)
	{
		authorized_admin.GET("/home", admin.ShowHome)
		authorized_admin.GET("/add-reg-question", admin.ShowAddRegQuestion)
		authorized_admin.POST("/add-reg-question", admin.HandleAddRegQuestion)
		authorized_admin.GET("/add-quiz-question", admin.ShowAddQuizQuestion)
		authorized_admin.POST("/add-quiz-question", admin.AddQuizQuestion)
		authorized_admin.GET("/reg-responses", admin.ShowRegResponses)
		authorized_admin.GET("/api/reg-responses", admin.GetRegResponsesData)
		authorized_admin.POST("/remove-duplicate-registrations", admin.RemoveDuplicateRegistrations)
		authorized_admin.POST("/assign-shifts", admin.AssignShiftsAndQuestions)
		authorized_admin.POST("/reassign-shift", admin.ReassignShiftAndQuestions)
		authorized_admin.GET("/api/signed-in-users", admin.ListSignedInUsers)
		authorized_admin.POST("/assign-slot", admin.AssignSlotToUser)
		authorized_admin.GET("/set-shift-timing", func(c *gin.Context) {
			c.HTML(200, "set_shift_timing.html", nil)
		})
		authorized_admin.POST("/set-shift-timing", admin.SetShiftTiming)
		authorized_admin.GET("/calcScore",quiz.GetScores)
	}

	view_admin := router.Group("/admin", admin.ViewAdminMiddleware)
	{
		view_admin.GET("/view", admin.ShowViewPage)
		view_admin.GET("/api/reg-responses-view", admin.GetRegResponsesDataSecond)
	}

	authorized_user := router.Group("/", login.AuthMiddleware)
	{
		authorized_user.GET("/regQuestions", registerationform.GetAllQues)
		authorized_user.POST("/register", registerationform.SubmitForm)
		authorized_user.GET("/quiz/get", quiz.GetQuizQues)
		authorized_user.POST("/quiz/submit", quiz.SubmitQuiz)
		// authorized_user.GET("/quiz/shifts", quiz.GetAppConfig)
		authorized_user.POST("/quiz/submitted", quiz.IsSubmitted)
		authorized_user.POST("/quiz/request-slot", quiz.RequestSlot)
		authorized_user.GET("/quiz/slot-status", quiz.SlotStatus)
	}


	// run on localhost:8080
	// router.Run(":8080")

	router.Run("0.0.0.0:2117")
}

func checkHealth(c *gin.Context) {
	// Report which build is actually serving. Render injects RENDER_GIT_COMMIT,
	// so this answers "did my deploy land?" without guessing from behaviour.
	commit := os.Getenv("RENDER_GIT_COMMIT")
	if commit == "" {
		commit = "local"
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Backend is up and running",
		"commit":  commit,
	})
}
