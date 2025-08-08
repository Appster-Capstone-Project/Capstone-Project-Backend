package listing

import (
    "net/http"
    "os"
    "fmt"

    "io"
    "github.com/gin-gonic/gin"
    	"github.com/albus-droid/Capstone-Project-Backend/internal/auth"
    "github.com/minio/minio-go/v7"
)


// RegisterRoutes mounts listing endpoints under /listings
func RegisterRoutes(r *gin.Engine, svc Service, minioClient *minio.Client) {
    // Public GET routes (no auth)
    public := r.Group("/listings")
    public.GET("", func(c *gin.Context) {
        if sellerID := c.Query("sellerId"); sellerID != "" {
            list, _ := svc.ListBySeller(sellerID)
            c.JSON(http.StatusOK, list)
            return
        }
        c.JSON(http.StatusOK, svc.ListAll())
    })

    public.GET("/:id", func(c *gin.Context) {
        id := c.Param("id")
        l, err := svc.GetByID(id)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
            return
        }
        c.JSON(http.StatusOK, l)
    })

    public.GET("/:id/image/:filename", func(c *gin.Context) {
        listingID := c.Param("id")
        filename := c.Param("filename")
        bucket := os.Getenv("MINIO_BUCKET")
        if bucket == "" {
            bucket = "listing-images"
        }
        objectName := fmt.Sprintf("listings/%s/%s", listingID, filename)

        // Stream the object via backend to avoid exposing MinIO host and CORS issues
        obj, err := minioClient.GetObject(c.Request.Context(), bucket, objectName, minio.GetObjectOptions{})
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch image"})
            return
        }
        defer obj.Close()

        // Fetch metadata to set headers
        info, err := obj.Stat()
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
            return
        }

        if info.ContentType != "" {
            c.Header("Content-Type", info.ContentType)
        } else {
            c.Header("Content-Type", "application/octet-stream")
        }
        c.Header("Cache-Control", "public, max-age=3600")
        c.Status(http.StatusOK)
        if _, err := io.Copy(c.Writer, obj); err != nil {
            // Best-effort; connection might be closed by client
            return
        }
    })

    protected := r.Group("/listings")
    protected.Use(auth.Middleware())

    // POST /listings
    protected.POST("", func(c *gin.Context) {
        var l Listing
        if err := c.ShouldBindJSON(&l); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        if err := svc.Create(&l); err != nil {
            c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusCreated, gin.H{"message": "listing created", "id": l.ID,})
    })

    // PUT /listings/:id  — update an existing listing
    protected.PUT("/:id", func(c *gin.Context) {
        id := c.Param("id")
        var l Listing
        if err := c.ShouldBindJSON(&l); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        if err := svc.Update(id, l); err != nil {
            // you can customize error handling based on your svc.Update error
            c.JSON(http.StatusNotFound, gin.H{"error": "not found or unable to update"})
            return
        }
        c.JSON(http.StatusOK, gin.H{"message": "listing updated"})
    })

    // DELETE /listings/:id  — remove a listing
    protected.DELETE("/:id", func(c *gin.Context) {
        id := c.Param("id")
        if err := svc.Delete(id); err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "not found or unable to delete"})
            return
        }
        c.Status(http.StatusNoContent)
    })


    // POST /listings/:id/image — Uploads image to MinIO and saves URL to Listing in DB
    protected.POST("/:id/image", func(c *gin.Context) {
        listingID := c.Param("id")
        file, header, err := c.Request.FormFile("file")
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "no file uploaded"})
            return
        }
        defer file.Close()

        bucket := os.Getenv("MINIO_BUCKET")
        if bucket == "" {
            bucket = "listing-images"
        }
        objectName := fmt.Sprintf("listings/%s/%s", listingID, header.Filename)
        contentType := header.Header.Get("Content-Type")

        _, err = minioClient.PutObject(
            c.Request.Context(), bucket, objectName, file, header.Size,
            minio.PutObjectOptions{ContentType: contentType},
        )
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "minio upload failed"})
            return
        }

        imageAPIUrl := fmt.Sprintf("/listings/%s/image/%s", listingID, header.Filename)

        // --- Update listing in DB ---
        listing, err := svc.GetByID(listingID)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
            return
        }

        // For single image per listing:
        listing.Image = imageAPIUrl

        // Save updated listing
        err = svc.Update(listingID, *listing)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update listing with image"})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "image_url": imageAPIUrl,
        })
    })
}