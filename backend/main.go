package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv"
	_ "github.com/lib/pq" // Важно: импортируем драйвер PostgreSQL, обязательно ставим _
	"github.com/rs/cors"
	_ "github.com/rs/cors"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/gomail.v2"
	"log"
	"net/http"
	_ "net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

var db *sql.DB
var jwtKey = []byte("my_secret_key")

// Таблица Users: id, username, password_hash, email, role
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// Таблица Algorithms: id, title, description, code, user_id, topic_id, rating, approved
// TODO: добавить поле difficulty, которое будет определять встроенный ИИ (в начале сложность передается просто в json)
type Algorithm struct {
	ID                  int       `json:"id"`
	Title               string    `json:"title"`
	Code                string    `json:"code"`
	UserID              int       `json:"user_id"`
	Topic               string    `json:"topic"`
	ProgrammingLanguage string    `json:"programming_language"`
	CreatedAt           time.Time `json:"created_at"`
}

type Claims struct {
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	jwt.StandardClaims
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateResetToken() (string, error) {
	token := make([]byte, 16)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func sendVerificationEmail(toEmail, username, verificationToken string) error {
	from := os.Getenv("EMAIL")
	password := os.Getenv("PASSWORD")
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		log.Fatal(err)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "Email Verification")

	appURL := os.Getenv("APP_URL")
	verificationURL := fmt.Sprintf("%s/verify-email?token=%s", appURL, verificationToken)
	emailBody := fmt.Sprintf("Dear %s,\n\nTo verify your email, please visit the following link:\n%s"+
		"\n\n\nIf this is not your nickname, please do NOT follow this link, otherwise you will register another user who specified your email address.",
		username, verificationURL)
	m.SetBody("text/plain", emailBody)

	d := gomail.NewDialer(smtpHost, smtpPort, from, password)

	if err := d.DialAndSend(m); err != nil {
		log.Println("Failed to send email:", err)
	} else {
		log.Println("Email sent successfully")
	}

	return err
}

func Register(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if user.Username == "" || user.Password == "" || user.Email == "" {
		http.Error(w, "All fields (username, password, email) must be provided", http.StatusBadRequest)
		return
	}

	if user.Password == user.Username || len(user.Password) < 4 {
		http.Error(w, "Password is too weak", http.StatusBadRequest)
		return
	}
	user.Role = "user"

	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", user.Username).Scan(&exists)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "Username already exists", http.StatusBadRequest)
		return
	}

	//log.Println("we are here : before checking confirmed email")

	var existsConfirmedEmail bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND confirmed = true)", user.Email).Scan(&existsConfirmedEmail)
	//log.Println("we are here : after checking confirmed email")
	//log.Println("error: ", err)
	//log.Println("existsConfirmedEmail: ", existsConfirmedEmail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if existsConfirmedEmail {
		http.Error(w, "User with this email already exists", http.StatusBadRequest)
		return
	}

	hashedPassword, err := hashPassword(user.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//log.Println("we are here : before inserting user")

	err = db.QueryRow("INSERT INTO users(username, password_hash, email, role) VALUES($1, $2, $3, $4) RETURNING id",
		user.Username, hashedPassword, user.Email, user.Role).Scan(&user.ID)
	//log.Println("we are here : after inserting user")
	//log.Println("error: ", err)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Генерируем токен верификации
	verificationToken, err := generateResetToken()
	//log.Println("verificationToken: ", verificationToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var existUserWithToken bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM email_verification_tokens WHERE user_id = $1)", user.ID).Scan(&existUserWithToken)
	//log.Println("existUserWithToken: ", existUserWithToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if existUserWithToken {
		//log.Println("User already have token")
		_, err = db.Exec("DELETE FROM email_verification_tokens WHERE user_id = $1", user.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	_, err = db.Exec("INSERT INTO email_verification_tokens(user_id, token, email, username) VALUES($1, $2, $3, $4)", user.ID, verificationToken, user.Email, user.Username)
	//log.Println("error: ", err)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = sendVerificationEmail(user.Email, user.Username, verificationToken)
	//log.Println("error: ", err)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user.Password = "" // Очищаем пароль перед возвратом данных пользователю
	json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful, please check your email to verify your account"})
}

func Login(w http.ResponseWriter, r *http.Request) {
	var creds User
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var storedUser User
	var confirmed bool = false
	err = db.QueryRow("SELECT id, username, password_hash, confirmed FROM users WHERE username=$1", creds.Username).
		Scan(&storedUser.ID, &storedUser.Username, &storedUser.Password, &confirmed)
	if err != nil {
		http.Error(w, "Invalid username", http.StatusUnauthorized)
		return
	}

	if !confirmed {
		http.Error(w, "Please verify your email before logging in", http.StatusUnauthorized)
		return
	}

	if !checkPasswordHash(creds.Password, storedUser.Password) {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(time.Hour)
	claims := &Claims{
		Username: storedUser.Username,
		UserID:   storedUser.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//http.SetCookie(w, &http.Cookie{
	//	Name:    "token",
	//	Value:   tokenString,
	//	Expires: expirationTime,
	//})

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login successful",
		"token":   tokenString,
		"userID":  strconv.Itoa(storedUser.ID),
	})
}

func VerifyEmail(w http.ResponseWriter, r *http.Request) {
	vars := r.URL.Query()
	token := vars.Get("token")

	log.Println("token: ", token)

	var userID int
	var email string
	err := db.QueryRow("SELECT user_id, email FROM email_verification_tokens WHERE token = $1", token).Scan(&userID, &email)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("UPDATE users SET confirmed = true WHERE id = $1", userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("DELETE FROM email_verification_tokens WHERE email = $1", email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("DELETE FROM users WHERE email = $1 AND confirmed = false", email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Email verified successfully"})
}

func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем заголовок Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Извлекаем токен из заголовка Authorization
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader { // проверяем, что токен корректно извлечен
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}

		tkn, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !tkn.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), "userID", claims.UserID))
		next.ServeHTTP(w, r)
	})
}

// ChangePassword Админ меняет пароль пользователю
//
//	TODO: Доступ должен быть к этой функции только у админа
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)

	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if user.Username == "" || user.Password == "" || user.Email == "" || user.Role == "" {
		http.Error(w, "All fields (username, password, email, role) must be provided", http.StatusBadRequest)
		return
	}

	if user.Password == user.Username || len(user.Password) < 4 {
		http.Error(w, "Password is too weak", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("userID").(int)
	currentPasswordHash, err := hashPassword(user.Password)
	result, err := db.Exec("UPDATE users SET password_hash = $1 WHERE id = $2", currentPasswordHash, userID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "No rows were updated", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Password changed"})
}

func sendResetPasswordEmail(toEmail, username, verificationToken string) error {
	from := os.Getenv("EMAIL")
	password := os.Getenv("PASSWORD")
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		log.Fatal(err)
	}

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "Reset Password")

	log.Println(username)

	emailBody := fmt.Sprintf("Dear " + username + ",\n\nTo reset your password, please copy the following token and paste it into the app:\n" + verificationToken +
		"\n\n\nIf this is not your nickname, please do NOT follow this link.")
	m.SetBody("text/plain", emailBody)

	d := gomail.NewDialer(smtpHost, smtpPort, from, password)

	if err := d.DialAndSend(m); err != nil {
		log.Println("Failed to send email:", err)
	} else {
		log.Println("Email sent successfully")
	}

	return err
}

func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	var storedUser User
	err = db.QueryRow("SELECT id, email, username FROM users WHERE username=$1", user.Username).Scan(&storedUser.ID, &storedUser.Email, &storedUser.Username)
	if err != nil {
		http.Error(w, "Invalid username", http.StatusUnauthorized)
		log.Println("Error fetching user from DB:", err)
		return
	}

	resetToken, err := generateResetToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error generating reset token:", err)
		return
	}

	_, err = db.Exec("DELETE FROM password_reset_tokens WHERE user_id = $1 AND email = $2", storedUser.ID, storedUser.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error deleting reset token into DB:", err)
		return
	}

	log.Println("user", user)

	_, err = db.Exec("INSERT INTO password_reset_tokens(user_id, token, email, username, created_at) VALUES($1, $2, $3, $4, $5)",
		storedUser.ID, resetToken, storedUser.Email, storedUser.Username, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error inserting reset token into DB:", err)
		return
	}

	log.Println("username", storedUser.Username)
	err = sendResetPasswordEmail(storedUser.Email, storedUser.Username, resetToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error sending reset email:", err)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Password reset email sent"})
	log.Println("Password reset email sent successfully to:", storedUser.Email)
}

func ResetPassword(w http.ResponseWriter, r *http.Request) {
	var RequestBody struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		Token       string `json:"token"`
		NewPassword string `json:"new-password"`
	}

	err := json.NewDecoder(r.Body).Decode(&RequestBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var userID int
	err = db.QueryRow("SELECT user_id FROM password_reset_tokens WHERE token=$1 AND username=$2 AND email=$3 AND created_at > $4",
		RequestBody.Token, RequestBody.Username, RequestBody.Email, time.Now().Add(-24*time.Hour)).Scan(&userID)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	hashedPassword, err := hashPassword(RequestBody.NewPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("UPDATE users SET password_hash=$1 WHERE id=$2", hashedPassword, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("DELETE FROM password_reset_tokens WHERE token=$1", RequestBody.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Password reset successful"})
}

// Handler to fetch programming languages
func GetAvailableProgrammingLanguages(w http.ResponseWriter, r *http.Request) {
	availableProgrammingLanguages := []string{"Go", "C++", "Python", "JavaScript",
		"Rust", "C#", "Java", "PHP", "Ruby", "Kotlin", "Swift", "C", "TypeScript", "Lua",
		"Haskell", "Lisp", "R", "Objective-C", "Scala", "Dart", "Elixir"}
	json.NewEncoder(w).Encode(availableProgrammingLanguages)
}

func CreateAlgorithm(w http.ResponseWriter, r *http.Request) {
	var algorithm Algorithm
	err := json.NewDecoder(r.Body).Decode(&algorithm)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if (algorithm.Topic == "") || (algorithm.ProgrammingLanguage == "") || (algorithm.Title == "") || (algorithm.Code == "") {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("userID").(int)
	algorithm.UserID = userID

	err = db.QueryRow("INSERT INTO algorithms(title, code, user_id, topic, programming_language) VALUES($1, $2, $3, $4, $5) RETURNING id",
		algorithm.Title, algorithm.Code, algorithm.UserID, algorithm.Topic, algorithm.ProgrammingLanguage).Scan(&algorithm.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(algorithm)
}

func UpdateAlgorithm(w http.ResponseWriter, r *http.Request) {
	var updateAlgorithm Algorithm
	err := json.NewDecoder(r.Body).Decode(&updateAlgorithm)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if (updateAlgorithm.Topic == "") || (updateAlgorithm.ProgrammingLanguage == "") || (updateAlgorithm.Title == "") || (updateAlgorithm.Code == "") {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("userID").(int)
	id := mux.Vars(r)["id"]

	result, err := db.Exec("UPDATE algorithms SET title = $1, code = $2, topic = $3, programming_language = $4 WHERE id = $5 AND user_id = $6",
		updateAlgorithm.Title, updateAlgorithm.Code, updateAlgorithm.Topic, updateAlgorithm.ProgrammingLanguage, id, userID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "No rows were updated", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(updateAlgorithm)
}

func GetAlgorithms(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")

	//log.Println("Authorization header:", authHeader)

	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	//log.Println("claims", claims)
	//log.Println("token", token)

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	//log.Println("now we go to fetching algorithms")

	// Fetch algorithms from database
	algorithms, err := db.Query("SELECT id, title, code, user_id, topic, programming_language FROM algorithms")
	if err != nil {
		http.Error(w, "Error fetching algorithms", http.StatusInternalServerError)
		return
	}
	defer algorithms.Close()

	var rows []map[string]interface{}
	for algorithms.Next() {
		var id int
		var title string
		var code string
		var userID int
		var topic string
		var programmingLanguage string

		err := algorithms.Scan(&id, &title, &code, &userID, &topic, &programmingLanguage)
		if err != nil {
			http.Error(w, "Error fetching algorithms", http.StatusInternalServerError)
			return
		}
		rows = append(rows, map[string]interface{}{
			"id":                   id,
			"title":                title,
			"code":                 code,
			"user_id":              userID,
			"topic":                topic,
			"programming_language": programmingLanguage,
		})
	}

	//log.Println("algorithms after fetching", rows)

	json.NewEncoder(w).Encode(rows)
}

func GetAlgorithmByID(w http.ResponseWriter, r *http.Request) {
	var algorithm Algorithm = Algorithm{}

	vars := mux.Vars(r)
	idStr, ok := vars["id"]

	if !ok {
		http.Error(w, "Missing ID parameter", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID parameter", http.StatusBadRequest)
		return
	}

	//log.Println("id of fetching algorithm by id: ", id)

	err = db.QueryRow("SELECT id, title, code, user_id, topic, programming_language FROM algorithms WHERE id = $1",
		id).Scan(&algorithm.ID, &algorithm.Title, &algorithm.Code, &algorithm.UserID, &algorithm.Topic, &algorithm.ProgrammingLanguage)

	//log.Println("algorithms after fetching by id", algorithm)
	//log.Println("error after fetching by id", err)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(algorithm)
}

func GetAlgorithmsByUserID(w http.ResponseWriter, r *http.Request) {
	var myAlgorithms []Algorithm

	userID, ok := r.Context().Value("userID").(int)
	if ok == false {
		http.Error(w, "Invalid userID", http.StatusBadRequest)
	}

	rows, err := db.Query("SELECT id, title, code, user_id, topic, programming_language FROM algorithms WHERE user_id = $1", userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer rows.Close()

	for rows.Next() {
		var algorithm Algorithm
		err := rows.Scan(&algorithm.ID, &algorithm.Title, &algorithm.Code, &algorithm.UserID, &algorithm.Topic, &algorithm.ProgrammingLanguage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		myAlgorithms = append(myAlgorithms, algorithm)
	}

	json.NewEncoder(w).Encode(myAlgorithms)
}

func GetAlgorithmsByFilter(w http.ResponseWriter, r *http.Request) {
	type filter struct {
		Topic               string `json:"topic"`
		ProgrammingLanguage string `json:"programming_language"`
		Title               string `json:"title"`
		AlgorithmID         int    `json:"id"`
		UserID              int    `json:"user_id"`
		SortBy              string `json:"sort_by"`
	}
	var filters filter

	params := r.URL.Query()

	filters.Title = params.Get("title")
	filters.Topic = params.Get("topic")
	filters.ProgrammingLanguage = params.Get("programming_language")
	filters.UserID, _ = strconv.Atoi(params.Get("user_id"))
	filters.AlgorithmID, _ = strconv.Atoi(params.Get("id"))
	filters.SortBy = params.Get("sort_by")

	//log.Println("filters: ", filters)

	query := "SELECT id, title, code, user_id, topic, programming_language FROM algorithms WHERE 1=1"
	var args []interface{}
	var argIndex int = 1

	if filters.Topic != "" {
		query += fmt.Sprintf(" AND topic ILIKE $%d", argIndex)
		args = append(args, "%"+filters.Topic+"%")
		argIndex++
	}
	if filters.ProgrammingLanguage != "" {
		query += fmt.Sprintf(" AND programming_language ILIKE $%d", argIndex)
		args = append(args, "%"+filters.ProgrammingLanguage+"%")
		argIndex++
	}
	if filters.Title != "" {
		query += fmt.Sprintf(" AND title ILIKE $%d", argIndex)
		args = append(args, "%"+filters.Title+"%")
		argIndex++
	}
	if filters.AlgorithmID != 0 {
		query += fmt.Sprintf(" AND id=$%d", argIndex)
		args = append(args, filters.AlgorithmID)
		argIndex++
	}
	if filters.UserID != 0 {
		query += fmt.Sprintf(" AND user_id=$%d", argIndex)
		args = append(args, filters.UserID)
		argIndex++
	}
	if filters.SortBy != "" {
		switch filters.SortBy {
		case "newest":
			query += " ORDER BY created_at DESC"
		case "most_popular":
			query += " ORDER BY rating DESC" // Assuming you have a rating field
		default:
			query += " ORDER BY created_at DESC"
		}
	}

	rows, err := db.Query(query, args...)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var algorithms []Algorithm = []Algorithm{}
	for rows.Next() {
		var algorithm Algorithm
		err := rows.Scan(&algorithm.ID, &algorithm.Title, &algorithm.Code, &algorithm.UserID, &algorithm.Topic, &algorithm.ProgrammingLanguage)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		algorithms = append(algorithms, algorithm)
	}

	json.NewEncoder(w).Encode(algorithms)
}

func main() {
	fmt.Println("Starting...")

	var err error
	db, err = sql.Open("postgres", "postgresql://postgres:postgres@localhost/postgres?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/register", Register).Methods("POST")
	router.HandleFunc("/verify-email", VerifyEmail).Methods("GET")
	router.HandleFunc("/login", Login).Methods("POST")

	router.HandleFunc("/forgot-password", ForgotPassword).Methods("POST")
	router.HandleFunc("/reset-password", ResetPassword).Methods("POST")

	protectedRoutes := router.PathPrefix("/api").Subrouter()
	protectedRoutes.Use(Authenticate)

	protectedRoutes.HandleFunc("/change-password", ChangePassword).Methods("PUT")

	protectedRoutes.HandleFunc("/available-programming-languages", GetAvailableProgrammingLanguages).Methods("GET")

	protectedRoutes.HandleFunc("/algorithms", CreateAlgorithm).Methods("POST")
	protectedRoutes.HandleFunc("/algorithms/{id}", UpdateAlgorithm).Methods("PUT")

	protectedRoutes.HandleFunc("/algorithms/search", GetAlgorithmsByFilter).Methods("GET")
	protectedRoutes.HandleFunc("/algorithms", GetAlgorithms).Methods("GET")
	protectedRoutes.HandleFunc("/algorithms/{id}", GetAlgorithmByID).Methods("GET")
	protectedRoutes.HandleFunc("/algorithms-by-user/{id}", GetAlgorithmsByUserID).Methods("GET")

	// Создаем новый CORS middleware с настройками по умолчанию
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Разрешаем все origins (для разработки); лучше ограничить в продакшн
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	// Используем CORS middleware для всех запросов
	handler := c.Handler(router)

	fmt.Println("Server is running on port 8081")
	log.Fatal(http.ListenAndServe(":8081", handler))
}
