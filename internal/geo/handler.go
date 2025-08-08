package geo

import (
    "net/http"
    "strconv"

    kd "github.com/albus-droid/Capstone-Project-Backend/internal/algorithms/kd-tree"
    "github.com/gin-gonic/gin"
)

// RegisterRoutes mounts geo endpoints under /geo
func RegisterRoutes(r *gin.Engine, svc *Service) {
    g := r.Group("/geo")

    // POST /geo/points -> body: [{"lon":..., "lat":...}, ...]
    g.POST("/points", func(c *gin.Context) {
        var pts []kd.Point
        if err := c.ShouldBindJSON(&pts); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        if len(pts) == 0 {
            c.JSON(http.StatusBadRequest, gin.H{"error": "no points provided"})
            return
        }
        if err := svc.SavePoints(c.Request.Context(), pts); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{"saved": len(pts)})
    })

    // GET /geo/query?lon=..&lat=..&radius_km=..
    g.GET("/query", func(c *gin.Context) {
        lonStr := c.Query("lon")
        latStr := c.Query("lat")
        radStr := c.Query("radius_km")
        if lonStr == "" || latStr == "" || radStr == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "lon, lat, radius_km are required"})
            return
        }
        lon, err1 := strconv.ParseFloat(lonStr, 64)
        lat, err2 := strconv.ParseFloat(latStr, 64)
        rad, err3 := strconv.ParseFloat(radStr, 64)
        if err1 != nil || err2 != nil || err3 != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lon/lat/radius_km"})
            return
        }
        pts, err := svc.Query(c.Request.Context(), lon, lat, rad)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusOK, gin.H{"count": len(pts), "points": pts})
    })
}