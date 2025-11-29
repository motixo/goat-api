package helper

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func GetStringFromContext(c *gin.Context, key string) (string, error) {
	val, exists := c.Get(key)
	if !exists {
		return "", fmt.Errorf("key %s not found in context", key)
	}

	result, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("key %s is not a string", key)
	}

	return result, nil
}
