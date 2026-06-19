//go:build sqlite && ignore

package main

import (
	"database/sql"
	"log"
	"math/rand"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL,
			age INTEGER,
			created_at TEXT,
			is_active BOOLEAN DEFAULT 1
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			category TEXT,
			price REAL,
			stock INTEGER DEFAULT 0,
			description TEXT
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			product_id INTEGER,
			quantity INTEGER,
			total_price REAL,
			order_date TEXT,
			status TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (product_id) REFERENCES products(id)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("DELETE FROM orders; DELETE FROM products; DELETE FROM users")
	if err != nil {
		log.Fatal(err)
	}

	usernames := []string{"alice", "bob", "charlie", "david", "emma", "frank", "grace", "henry", "ivy", "jack"}
	categories := []string{"Electronics", "Books", "Clothing", "Food", "Sports", "Home", "Beauty", "Toys"}
	productNames := []string{"Laptop", "Smartphone", "Headphones", "Novel", "T-shirt", "Coffee", "Basketball", "Lamp", "Shampoo", "Doll"}
	statuses := []string{"pending", "shipped", "delivered", "cancelled"}

	for _, name := range usernames {
		createdAt := time.Now().AddDate(0, 0, -rand.Intn(365)).Format(time.RFC3339)
		_, err = db.Exec(
			"INSERT INTO users (username, email, age, created_at, is_active) VALUES (?, ?, ?, ?, ?)",
			name, name+"@example.com", 20+rand.Intn(50), createdAt, rand.Intn(2) == 0,
		)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Inserted user: %s", name)
	}

	for i, name := range productNames {
		category := categories[i%len(categories)]
		price := float64(10+rand.Intn(990)) + rand.Float64()*0.99
		_, err = db.Exec(
			"INSERT INTO products (name, category, price, stock, description) VALUES (?, ?, ?, ?, ?)",
			name, category, price, rand.Intn(500), "This is a high-quality "+name,
		)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Inserted product: %s ($%.2f)", name, price)
	}

	for i := 0; i < 50; i++ {
		userID := 1 + rand.Intn(len(usernames))
		productID := 1 + rand.Intn(len(productNames))
		quantity := 1 + rand.Intn(10)
		price := float64(10+rand.Intn(990)) + rand.Float64()*0.99
		total := float64(quantity) * price
		orderDate := time.Now().AddDate(0, 0, -rand.Intn(90)).Format(time.RFC3339)
		status := statuses[rand.Intn(len(statuses))]

		_, err = db.Exec(
			"INSERT INTO orders (user_id, product_id, quantity, total_price, order_date, status) VALUES (?, ?, ?, ?, ?, ?)",
			userID, productID, quantity, total, orderDate, status,
		)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Println("Inserted 50 orders")

	log.Println("\nDatabase initialized successfully!")
	log.Println("\nSample queries to try:")
	log.Println("  SELECT * FROM users")
	log.Println("  SELECT COUNT(*) as total FROM orders")
	log.Println("  SELECT u.username, COUNT(o.id) as order_count FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id")
	log.Println("  SELECT category, AVG(price) as avg_price FROM products GROUP BY category")
}
