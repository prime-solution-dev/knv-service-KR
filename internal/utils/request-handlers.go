package utils

import (
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ProcessRequestPayload(c *gin.Context, serviceFunc func(*gin.Context, string) (interface{}, error)) {

	jsonData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := serviceFunc(c, string(jsonData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

func ProcessRequestMultiPart(c *gin.Context, serviceFunc func(*gin.Context) (interface{}, error)) {
	form, err := c.MultipartForm()
	if err != nil {
		log.Println("Error parsing multipart form:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get multipart form: " + err.Error()})
		return
	}

	for fieldName, fileHeaders := range form.File {
		for _, fileHeader := range fileHeaders {
			log.Println("Field Name:", fieldName)
			log.Println("Uploaded File:", fileHeader.Filename)
		}
	}

	response, err := serviceFunc(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}
