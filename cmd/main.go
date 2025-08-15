package main

import (
	"github.com/albus-droid/Capstone-Project-Backend/internal/auth"
	"github.com/albus-droid/Capstone-Project-Backend/internal/db"
	"github.com/albus-droid/Capstone-Project-Backend/internal/image_store"
	"github.com/albus-droid/Capstone-Project-Backend/internal/listing"
	"github.com/albus-droid/Capstone-Project-Backend/internal/notification"
	"github.com/albus-droid/Capstone-Project-Backend/internal/order"
	"github.com/albus-droid/Capstone-Project-Backend/internal/seller"
	"github.com/albus-droid/Capstone-Project-Backend/internal/user"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	db := db.Init()
	redisStore := auth.NewRedisStore("redis:6379", "", 0)
	minioClient, err := image_store.NewMinioClientFromEnv()
	if err != nil {
		panic(err)
	}

	// user routes
	user.Migrate(db) // optional for dev
	usvc := user.NewPostgresService(db)
	user.RegisterRoutes(r, usvc, redisStore)

	// seller routes
	seller.Migrate(db) // optional for dev
	ssvc := seller.NewPostgresService(db)
	seller.RegisterRoutes(r, ssvc, redisStore)

	// Listing routes
	listing.Migrate(db) // optional for dev
	lsvc := listing.NewPostgresService(db)
	listing.RegisterRoutes(r, lsvc, minioClient)

	// Order
	order.Migrate(db) // optional for dev
	osvc := order.NewPostgresService(db)
	order.RegisterRoutes(r, osvc)

	nm := notification.NewManager(db)
	nm.RegisterRoutes(r)
	go nm.Run()

	r.Run(":8000") // http://localhost:8080
}
