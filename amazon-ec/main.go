package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	_ "golang.org/x/image/webp"
	"golang.org/x/term"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	_ "modernc.org/sqlite"
)

type Product struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Price         int    `json:"price"`
	Stock         int    `json:"stock"`
	ImageFilename string `json:"image_filename"`
	ImageURL      string `json:"image_url"`
	IsActive      int    `json:"is_active"`
}

type OrderRequest struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

type OrderResponse struct {
	ID             int    `json:"id"`
	ProductName    string `json:"product_name"`
	Price          int    `json:"price"`
	Quantity       int    `json:"quantity"`
	CreatedAt      string `json:"created_at"`
	ShippingStatus string `json:"shipping_status"`
}

type PaymentRequest struct {
	OrderIDs []int  `json:"order_ids"`
	Method   string `json:"method"`
}

var db *sql.DB

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--set-admin" {
		setAdminCommand()
		return
	}
	var err error
	db, err = sql.Open("sqlite", "./ec.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	initDB()

	// APIルーティング
	http.HandleFunc("/api/products", getProductsHandler)
	http.HandleFunc("/api/product", getProductDetailHandler)
	http.HandleFunc("/api/orders", ordersHandler)
	http.HandleFunc("/api/payments", createPaymentHandler)
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/me", meHandler)
	http.HandleFunc("/api/logout", logoutHandler)
	http.HandleFunc("/api/admin/products", adminProductsHandler)
	http.HandleFunc("/api/admin/product", adminProductHandler)
	http.HandleFunc("/api/admin/upload", adminUploadHandler)
	http.HandleFunc("/api/admin/shipments", adminShipmentsHandler)
	http.HandleFunc("/api/admin/shipments/dispatch", adminDispatchHandler)
	http.HandleFunc("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./static/uploads"))).ServeHTTP)

	// 静的ファイルサーバー
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	port := ":8080"
	fmt.Printf("Server starting on http://localhost%s ...\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func initDB() {
	createProductsTable := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		price INTEGER NOT NULL
	);`
	_, err := db.Exec(createProductsTable)
	if err != nil {
		log.Fatal("Failed to create products table:", err)
	}

	createOrdersTable := `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		product_id INTEGER NOT NULL,
		quantity INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createOrdersTable)
	if err != nil {
		log.Fatal("Failed to create orders table:", err)
	}

	createPaymentsTable := `
	CREATE TABLE IF NOT EXISTS payments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
        order_id INTEGER NOT NULL,
		amount INTEGER NOT NULL,
		method TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createPaymentsTable)
	if err != nil {
		log.Fatal("Failed to create payments table:", err)
	}

	// 既存データを保持し、顧客用のテーブルと列だけを追加する。
	for _, query := range []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT UNIQUE NOT NULL, password_hash TEXT NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, user_id INTEGER NOT NULL, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if _, err := db.Exec(query); err != nil {
			log.Fatal("顧客テーブル作成失敗: ", err)
		}
	}
	var userIDColumns int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('orders') WHERE name = 'user_id'").Scan(&userIDColumns); err != nil {
		log.Fatal(err)
	}
	if userIDColumns == 0 {
		if _, err := db.Exec("ALTER TABLE orders ADD COLUMN user_id INTEGER"); err != nil {
			log.Fatal("注文テーブル移行失敗: ", err)
		}
	}
	// 第22回: 既存データを消さずに商品・管理者列を追加する。
	for _, col := range []struct{ table, name, definition string }{
		{"products", "stock", "INTEGER NOT NULL DEFAULT 0"},
		{"products", "image_filename", "TEXT"},
		{"products", "is_active", "INTEGER NOT NULL DEFAULT 1"},
		{"users", "is_admin", "INTEGER NOT NULL DEFAULT 0"},
	} {
		var exists int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('"+col.table+"') WHERE name = ?", col.name).Scan(&exists); err != nil {
			log.Fatal(err)
		}
		if exists == 0 {
			if _, err := db.Exec("ALTER TABLE " + col.table + " ADD COLUMN " + col.name + " " + col.definition); err != nil {
				log.Fatal("移行失敗: ", err)
			}
		}
	}
	// 第23回: 既存注文を保持したまま配送状態列を追加する。
	var shippingColumn int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('orders') WHERE name = ?", "shipping_status").Scan(&shippingColumn); err != nil {
		log.Fatal(err)
	}
	if shippingColumn == 0 {
		if _, err := db.Exec("ALTER TABLE orders ADD COLUMN shipping_status TEXT NOT NULL DEFAULT '受付'"); err != nil {
			log.Fatal("配送列移行失敗: ", err)
		}
	}
	// 古いpaymentsテーブルを自動削除・変更しない。
	var columnCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('payments') WHERE name = 'order_id'").Scan(&columnCount); err != nil || columnCount != 1 {
		log.Fatal("paymentsテーブルにorder_idがありません。ec.dbをバックアップし、移行手順を確認してください")
	}
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if err == nil && count == 0 {
		insertQuery := `INSERT INTO products (name, price) VALUES 
			('ノートパソコン', 120000),
			('ワイヤレスマウス', 3500),
			('USBメモリ 64GB', 1200);`
		_, err = db.Exec(insertQuery)
		if err != nil {
			log.Println("Failed to insert initial products:", err)
		} else {
			log.Println("Initial products inserted successfully.")
		}
	}
}

func getProductsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := db.Query("SELECT id, name, price, stock, COALESCE(image_filename, ''), is_active FROM products WHERE is_active=1")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.ImageFilename, &p.IsActive); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p.ImageURL = productImageURL(p.ImageFilename)
		products = append(products, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func getProductDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid id parameter", http.StatusBadRequest)
		return
	}

	var p Product
	err = db.QueryRow("SELECT id, name, price, stock, COALESCE(image_filename, ''), is_active FROM products WHERE id = ? AND is_active=1", id).Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.ImageFilename, &p.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Product not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	p.ImageURL = productImageURL(p.ImageFilename)
	json.NewEncoder(w).Encode(p)
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		createOrderHandler(w, r)
	case http.MethodGet:
		getOrdersHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.ProductID <= 0 || req.Quantity <= 0 {
		http.Error(w, "Invalid product_id or quantity", http.StatusBadRequest)
		return
	}

	stmt, err := db.Prepare("INSERT INTO orders (product_id, quantity, user_id) VALUES (?, ?, ?)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	var userID interface{}
	if id, ok := currentUserID(r); ok {
		userID = id
	}
	result, err := stmt.Exec(req.ProductID, req.Quantity, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "注文ID取得失敗", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "message": "注文しました"})
}

func getOrdersHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		http.Error(w, "ログインが必要です", http.StatusUnauthorized)
		return
	}
	rows, err := db.Query(`SELECT orders.id, products.name, products.price, orders.quantity, orders.created_at, orders.shipping_status
 FROM orders JOIN products ON orders.product_id = products.id
 WHERE orders.user_id = ? ORDER BY orders.id DESC`, userID)
	if err != nil {
		http.Error(w, "注文履歴取得失敗", 500)
		return
	}
	defer rows.Close()
	orders := []OrderResponse{}
	for rows.Next() {
		var o OrderResponse
		if err := rows.Scan(&o.ID, &o.ProductName, &o.Price, &o.Quantity, &o.CreatedAt, &o.ShippingStatus); err != nil {
			http.Error(w, "注文履歴取得失敗", 500)
			return
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "注文履歴取得失敗", 500)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func createPaymentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", 400)
		return
	}
	if len(req.OrderIDs) == 0 || (req.Method != "credit" && req.Method != "bank" && req.Method != "cod") {
		http.Error(w, "Invalid payment parameters", 400)
		return
	}
	seen := map[int]bool{}
	for _, id := range req.OrderIDs {
		if id <= 0 || seen[id] {
			http.Error(w, "Invalid order IDs", 400)
			return
		}
		seen[id] = true
	}
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Transaction error", 500)
		return
	}
	defer tx.Rollback()
	for _, id := range req.OrderIDs {
		var amount int64
		err := tx.QueryRow(`SELECT CAST(products.price AS INTEGER) * CAST(orders.quantity AS INTEGER)
   FROM orders JOIN products ON orders.product_id = products.id WHERE orders.id = ?`, id).Scan(&amount)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Order not found", 404)
			return
		}
		if err != nil || amount <= 0 {
			http.Error(w, "Invalid order amount", 400)
			return
		}
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM payments WHERE order_id = ?", id).Scan(&count); err != nil {
			http.Error(w, "Payment check failed", 500)
			return
		}
		if count != 0 {
			http.Error(w, "Already paid", 409)
			return
		}
		if _, err := tx.Exec("INSERT INTO payments (order_id, amount, method, status) VALUES (?, ?, ?, 'paid')", id, amount, req.Method); err != nil {
			http.Error(w, "Payment insert failed", 500)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "Payment commit failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "決済が完了しました", "status": "paid"})
}

// Cookieからセッションを検証する。ログインしていない場合はfalse。
func currentUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil || cookie.Value == "" {
		return 0, false
	}
	var id int64
	err = db.QueryRow("SELECT user_id FROM sessions WHERE token = ?", cookie.Value).Scan(&id)
	return id, err == nil
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 8 || len([]byte(req.Password)) > 72 {
		http.Error(w, "ユーザー名と8～72バイトのパスワードを入力してください", 400)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "登録処理失敗", 500)
		return
	}
	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", req.Username, string(hash))
	if err != nil {
		http.Error(w, "ユーザー名が使用できません", 409)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "会員登録しました"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}
	var id int64
	var hash string
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", strings.TrimSpace(req.Username)).Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "ユーザー名またはパスワードが違います", 401)
		return
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		http.Error(w, "セッション作成失敗", 500)
		return
	}
	token := hex.EncodeToString(random)
	if _, err := db.Exec("INSERT INTO sessions (token, user_id) VALUES (?, ?)", token, id); err != nil {
		http.Error(w, "セッション保存失敗", 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ログインしました"})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	id, ok := currentUserID(r)
	if !ok {
		http.Error(w, "ログインしていません", 401)
		return
	}
	var username string
	if err := db.QueryRow("SELECT username FROM users WHERE id = ?", id).Scan(&username); err != nil {
		http.Error(w, "ユーザーが見つかりません", 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var isAdmin int
	if err := db.QueryRow("SELECT is_admin FROM users WHERE id=?", id).Scan(&isAdmin); err != nil {
		http.Error(w, "認証エラー", 401)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "username": username, "is_admin": isAdmin == 1})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if cookie, err := r.Cookie("session_id"); err == nil {
		db.Exec("DELETE FROM sessions WHERE token = ?", cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "ログアウトしました"})
}

// ===== 第22回 商品管理 =====
func productImageURL(filename string) string {
	if filename == "" {
		return ""
	}
	return "/uploads/" + url.PathEscape(filename)
}
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	id, ok := currentUserID(r)
	if !ok {
		http.Error(w, "ログインが必要です", http.StatusUnauthorized)
		return false
	}
	var admin int
	if err := db.QueryRow("SELECT is_admin FROM users WHERE id=?", id).Scan(&admin); err != nil || admin != 1 {
		http.Error(w, "管理者専用です", http.StatusForbidden)
		return false
	}
	// 管理操作はCookieを使うため、同一Originからの要求のみ許可する。
	if r.Method != http.MethodGet {
		origin := r.Header.Get("Origin")
		if origin == "" {
			http.Error(w, "Originヘッダーが必要です", 403)
			return false
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "Originが一致しません", 403)
			return false
		}
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func adminProductsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query("SELECT id,name,price,stock,COALESCE(image_filename,''),is_active FROM products ORDER BY id DESC")
		if err != nil {
			http.Error(w, "取得失敗", 500)
			return
		}
		defer rows.Close()
		products := []Product{}
		for rows.Next() {
			var p Product
			if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock, &p.ImageFilename, &p.IsActive); err != nil {
				http.Error(w, "取得失敗", 500)
				return
			}
			p.ImageURL = productImageURL(p.ImageFilename)
			products = append(products, p)
		}
		writeJSON(w, 200, products)
	case http.MethodPost:
		var v struct {
			Name          string `json:"name"`
			Price         int    `json:"price"`
			Stock         int    `json:"stock"`
			ImageFilename string `json:"image_filename"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&v); err != nil {
			http.Error(w, "入力形式が不正です", 400)
			return
		}
		v.Name = strings.TrimSpace(v.Name)
		if !validProduct(v.Name, v.Price, v.Stock, v.ImageFilename) {
			http.Error(w, "商品情報が不正です", 400)
			return
		}
		res, err := db.Exec("INSERT INTO products(name,price,stock,image_filename,is_active) VALUES(?,?,?,?,1)", v.Name, v.Price, v.Stock, nullableFilename(v.ImageFilename))
		if err != nil {
			http.Error(w, "登録失敗", 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 201, map[string]interface{}{"id": id})
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
func nullableFilename(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
func validProduct(name string, price, stock int, filename string) bool {
	if name == "" || len([]byte(name)) > 200 || price < 0 || price > 1000000000 || stock < 0 || stock > 1000000000 {
		return false
	}
	if filename != "" {
		if filepath.Base(filename) != filename || strings.ContainsAny(filename, "/\\") {
			return false
		}
		if strings.HasSuffix(filename, ".webp") {
			if len(filename) != 37 {
				return false
			}
		} else if len(filename) != 36 {
			return false
		}
		if !strings.HasSuffix(filename, ".jpg") && !strings.HasSuffix(filename, ".png") && !strings.HasSuffix(filename, ".webp") {
			return false
		}
		if _, err := hex.DecodeString(filename[:32]); err != nil {
			return false
		}
	}
	return true
}
func adminProductHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil || id <= 0 {
		http.Error(w, "IDが不正です", 400)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var v struct {
			Name          string `json:"name"`
			Price         int    `json:"price"`
			Stock         int    `json:"stock"`
			ImageFilename string `json:"image_filename"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&v); err != nil {
			http.Error(w, "入力形式が不正です", 400)
			return
		}
		v.Name = strings.TrimSpace(v.Name)
		if !validProduct(v.Name, v.Price, v.Stock, v.ImageFilename) {
			http.Error(w, "商品情報が不正です", 400)
			return
		}
		res, err := db.Exec("UPDATE products SET name=?,price=?,stock=?,image_filename=? WHERE id=? AND is_active=1", v.Name, v.Price, v.Stock, nullableFilename(v.ImageFilename), id)
		if err != nil {
			http.Error(w, "更新失敗", 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			http.Error(w, "商品がありません", 404)
			return
		}
		writeJSON(w, 200, map[string]string{"message": "更新しました"})
	case http.MethodDelete:
		res, err := db.Exec("UPDATE products SET is_active=0 WHERE id=? AND is_active=1", id)
		if err != nil {
			http.Error(w, "非表示処理失敗", 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			http.Error(w, "商品がありません", 404)
			return
		}
		writeJSON(w, 200, map[string]string{"message": "非表示にしました"})
	default:
		http.Error(w, "Method not allowed", 405)
	}
}
func adminUploadHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	const maxImage = 5 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxImage+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "ファイルが大きすぎるか形式が不正です", 400)
		return
	}
	f, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "画像がありません", 400)
		return
	}
	defer f.Close()
	if header.Size <= 0 || header.Size > maxImage {
		http.Error(w, "画像は5MB以下にしてください", 400)
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, maxImage+1))
	if err != nil || len(data) > maxImage {
		http.Error(w, "画像が大きすぎます", 400)
		return
	}
	mime := http.DetectContentType(data)
	ext := ""
	switch mime {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		http.Error(w, "JPEG・PNG・WebPのみ使用できます", 400)
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8000 || cfg.Height > 8000 {
		http.Error(w, "画像データが不正です", 400)
		return
	}
	if (mime == "image/jpeg" && format != "jpeg") || (mime == "image/png" && format != "png") || (mime == "image/webp" && format != "webp") {
		http.Error(w, "画像形式が一致しません", 400)
		return
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "保存失敗", 500)
		return
	}
	filename := hex.EncodeToString(b) + ext
	if err := os.MkdirAll("static/uploads", 0755); err != nil {
		http.Error(w, "保存失敗", 500)
		return
	}
	out, err := os.OpenFile(filepath.Join("static/uploads", filename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		http.Error(w, "保存失敗", 500)
		return
	}
	_, err = out.Write(data)
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		os.Remove(filepath.Join("static/uploads", filename))
		http.Error(w, "保存失敗", 500)
		return
	}
	writeJSON(w, 201, map[string]string{"image_filename": filename, "image_url": productImageURL(filename)})
}
func setAdminCommand() {
	// 実行例: go run main.go --set-admin admin1
	if len(os.Args) != 3 {
		log.Fatal("使用方法: go run main.go --set-admin ユーザー名")
	}
	name := strings.TrimSpace(os.Args[2])
	if name == "" {
		log.Fatal("ユーザー名を指定してください")
	}
	var err error
	db, err = sql.Open("sqlite", "./ec.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	initDB()
	fmt.Print("管理者パスワード（画面には表示されません）: ")
	pass, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		for i := range pass {
			pass[i] = 0
		}
	}()
	if len(pass) < 12 || len(pass) > 72 {
		log.Fatal("管理者パスワードは12～72バイトにしてください")
	}
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	// 既存の一般顧客を昇格させず、管理者専用ユーザーを新規作成する。
	_, err = db.Exec("INSERT INTO users(username,password_hash,is_admin) VALUES(?,?,1)", name, string(hash))
	if err != nil {
		log.Fatal("管理者を作成できません。別のユーザー名を指定してください: ", err)
	}
	fmt.Println("管理者アカウントを作成しました。通常起動は go run main.go です")
}

// ===== 第23回 配送管理 =====
type ShipmentResponse struct {
	ID             int64  `json:"id"`
	ProductName    string `json:"product_name"`
	Quantity       int    `json:"quantity"`
	CreatedAt      string `json:"created_at"`
	ShippingStatus string `json:"shipping_status"`
	PaymentStatus  string `json:"payment_status"`
}

func adminShipmentsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	rows, err := db.Query(`SELECT o.id,p.name,o.quantity,o.created_at,o.shipping_status,
 CASE WHEN EXISTS(SELECT 1 FROM payments pay WHERE pay.order_id=o.id AND pay.status='paid') THEN 'paid' ELSE 'unpaid' END
 FROM orders o JOIN products p ON p.id=o.product_id ORDER BY o.id DESC`)
	if err != nil {
		http.Error(w, "配送一覧取得失敗", 500)
		return
	}
	defer rows.Close()
	items := []ShipmentResponse{}
	for rows.Next() {
		var item ShipmentResponse
		if err := rows.Scan(&item.ID, &item.ProductName, &item.Quantity, &item.CreatedAt, &item.ShippingStatus, &item.PaymentStatus); err != nil {
			http.Error(w, "配送一覧取得失敗", 500)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "配送一覧取得失敗", 500)
		return
	}
	writeJSON(w, 200, items)
}
func adminDispatchHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct {
		OrderID int64 `json:"order_id"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.OrderID <= 0 {
		http.Error(w, "有効なorder_idを指定してください", 400)
		return
	}
	// 単一UPDATEで決済済み・未発送・存在する注文だけを原子的に更新する。
	result, err := db.Exec(`UPDATE orders SET shipping_status='発送済み'
 WHERE id=? AND shipping_status='受付'
 AND EXISTS(SELECT 1 FROM payments WHERE payments.order_id=orders.id AND payments.status='paid')`, req.OrderID)
	if err != nil {
		http.Error(w, "発送更新失敗", 500)
		return
	}
	n, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "発送更新確認失敗", 500)
		return
	}
	if n != 1 {
		http.Error(w, "注文が存在しない、未決済、または発送済みです", 409)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"order_id": req.OrderID, "shipping_status": "発送済み"})
}
