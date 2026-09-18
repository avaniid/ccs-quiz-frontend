package admin

import (
	"crypto/rand"
	"net/http"
	"os"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
)

var validationRegexMap = map[string]string{
	"string":     ".*",
	"email":      `^[\w-\.]+@([\w-]+\.)+[\w-]{2,4}$`,
	"number":     `^\d+$`,
	"github":     `^(https?:\/\/)?(www\.)?github\.com\/[A-Za-z0-9-]{1,39}\/?$`,
	"linkedin":   `^(https?:\/\/)?(www\.)?linkedin\.com\/in\/[A-Za-z0-9_-]+\/?$`,
	"codechef":   `^(https?:\/\/)?(www\.)?codechef\.com\/users\/[A-Za-z0-9_-]+\/?$`,
	"codeforces": `^(https?:\/\/)?(www\.)?codeforces\.com\/profile\/[A-Za-z0-9_-]+\/?$`,
	"leetcode":   `^(https?:\/\/)?(www\.)?leetcode\.com\/u\/[A-Za-z0-9_-]{3,20}\/?$`,
	"link":       `^(https?:\/\/)?([\w\-])+\.{1}([a-zA-Z]{2,63})([\/\w\-\.\?=%&]*)*\/?$`,
}

func GenerateQID() models.QID {
	var id [7]byte
	_, err := rand.Read(id[:])
	if err != nil {
		panic("failed to generate QID")
	}
	return &id
}

func AddRegQuestion(q *models.Questions, questiontype bool) error {

	if q.QuestionID == nil {
		q.QuestionID = GenerateQID()
	}
	_, err := db.Registeration_Questions.Coll.InsertOne(db.Registeration_Questions.Context, q)
	return err

}

func MainAdminMiddleware(c *gin.Context) {
	auth, err := c.Cookie("admin_auth")
	if err != nil || auth != "main" {
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
		return
	}
	// c.Redirect(http.StatusFound, "/admin/home")
	c.Next()
}

func ViewAdminMiddleware(c *gin.Context) {
	auth, err := c.Cookie("admin_auth")
	if err != nil || auth != "view" {
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
		return
	}
	// c.Redirect(http.StatusFound, "/admin/view")
	c.Next()
}

func ShowViewPage(c *gin.Context) {
	c.HTML(http.StatusOK, "view.html", nil)
}

func ShowHome(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func HandleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	user1 := os.Getenv("ADMIN_USER1")
	pass1 := os.Getenv("ADMIN_PASS1")
	user2 := os.Getenv("ADMIN_USER2")
	pass2 := os.Getenv("ADMIN_PASS2")

	// Without this, unset ADMIN_USER1/ADMIN_PASS1 leave user1 and pass1 as ""
	// and an empty login form matches them — handing full admin access to
	// anyone who just presses Submit.
	if username == "" || password == "" {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid credentials"})
		return
	}

	if user1 != "" && username == user1 && password == pass1 {
		c.SetCookie("admin_auth", "main", 3600, "/", "", true, true)
		c.Redirect(http.StatusFound, "/admin/home")
		return
	}
	if user2 != "" && username == user2 && password == pass2 {
		c.SetCookie("admin_auth", "view", 3600, "/", "", true, true)
		c.Redirect(http.StatusFound, "/admin/view")
		return
	}
	c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid credentials"})
}

func ShowAddRegQuestion(c *gin.Context) {
	c.HTML(http.StatusOK, "add_reg_question.html", nil)
}

func HandleAddRegQuestion(c *gin.Context) {
	question := c.PostForm("question")
	qtype := c.PostForm("questiontype") == "true"
	validationType := c.PostForm("validation")
	regex, ok := validationRegexMap[validationType]
	if !ok {
		regex = ".*"
	}

	q := &models.Questions{
		Question:     question,
		QuestionType: qtype,
		Validation:   regex,
	}

	if err := AddRegQuestion(q, qtype); err != nil {
		c.HTML(http.StatusInternalServerError, "add_reg_question.html", gin.H{"Error": "Failed to add question"})
		return
	}

	c.HTML(http.StatusOK, "add_reg_question.html", gin.H{"Message": "Question added successfully"})
}
