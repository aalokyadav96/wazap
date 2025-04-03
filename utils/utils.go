package utils

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	rndm "math/rand"
	"net/http"
	"nwr/globals"
	"nwr/middleware"
	"os"

	"mime/multipart"

	"slices"

	"github.com/disintegration/imaging"
	"github.com/golang-jwt/jwt/v5"
	"github.com/julienschmidt/httprouter"
)

func CSRF(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, GenerateStringName(8))
}

func GenerateStringName(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rndm.Intn(len(letters))]
	}
	return string(b)
}

func GenerateIntID(n int) string {
	var letters = []rune("0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rndm.Intn(len(letters))]
	}
	return string(b)
}

func EncrypIt(strToHash string) string {
	data := []byte(strToHash)
	return fmt.Sprintf("%x", md5.Sum(data))
}

func SendResponse(w http.ResponseWriter, status int, data any, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]any{
		"status":  status,
		"message": message,
		"data":    data,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	// Encode response and check for encoding errors
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Helper function to check if a user is in a slice of followers
func Contains(slice []string, value string) bool {
	return slices.Contains(slice, value)
}

// Utility function to send JSON response
func SendJSONResponse(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// List of supported image MIME types
var SupportedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
	"image/bmp":  true,
	"image/tiff": true,
}

func ValidateImageFileType(w http.ResponseWriter, header *multipart.FileHeader) bool {
	mimeType := header.Header.Get("Content-Type")

	if !SupportedImageTypes[mimeType] {
		http.Error(w, "Invalid file type. Supported formats: JPEG, PNG, WebP, GIF, BMP, TIFF, SVG.", http.StatusBadRequest)
		return false
	}

	return true
}

func CreateThumb(filename string, fileLocation string, fileType string, thumbWidth int, thumbHeight int) error {
	inputPath := fmt.Sprintf("%s/%s%s", fileLocation, filename, fileType)
	outputPath := fmt.Sprintf("%s/thumb/%s%s", fileLocation, filename, fileType)

	// Ensure directory exists
	if err := ensureDir(fileLocation); err != nil {
		log.Println("failed to create upload directory: %w", err)
	}

	fmt.Println(outputPath)
	// thumbWidth := 300
	// thumbHeight := 200
	bgColor := color.White // Change to color.Transparent for a transparent background

	// Open the original image
	img, err := imaging.Open(inputPath)
	if err != nil {
		return err
	}

	// Get the original dimensions
	origWidth := img.Bounds().Dx()
	origHeight := img.Bounds().Dy()

	// Calculate new size while maintaining aspect ratio
	newWidth, newHeight := fitResolution(origWidth, origHeight, thumbWidth, thumbHeight)

	// Resize the image
	resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Create a new blank image with the target thumbnail size and a background color
	thumbImg := imaging.New(thumbWidth, thumbHeight, bgColor)

	// Calculate the position to center the resized image
	xPos := (thumbWidth - newWidth) / 2
	yPos := (thumbHeight - newHeight) / 2

	// Paste the resized image onto the blank canvas
	thumbImg = imaging.Paste(thumbImg, resizedImg, image.Pt(xPos, yPos))

	// Save the final thumbnail
	return imaging.Save(thumbImg, outputPath)
}

func fitResolution(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
	// If the original image is already smaller than the target size, keep it unchanged
	if origWidth <= maxWidth && origHeight <= maxHeight {
		return origWidth, origHeight
	}

	// Calculate the scaling factor for both width and height
	widthRatio := float64(maxWidth) / float64(origWidth)
	heightRatio := float64(maxHeight) / float64(origHeight)

	// Use the smaller ratio to ensure the image fits within bounds
	scaleFactor := math.Min(widthRatio, heightRatio)

	// Compute new dimensions
	newWidth := int(float64(origWidth) * scaleFactor)
	newHeight := int(float64(origHeight) * scaleFactor)

	return newWidth, newHeight
}

// Generic function to ensure directory existence
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func ValidateJWT(tokenString string) (*middleware.Claims, error) {
	if tokenString == "" || len(tokenString) < 8 {
		return nil, fmt.Errorf("invalid token")
	}

	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}
	return claims, nil
}
