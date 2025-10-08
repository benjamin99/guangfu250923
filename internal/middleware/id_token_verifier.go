import (
	"github.com/gin-gonic/gin"
)

type idTokenInput struct {
	IdToken string `json:"id_token"`
}

// IdTokenVerifier verifies the id token from the request body
// *NOTE* since we parse the body in the middleware, we need to use ShouldBindBodyWithJSON instead of ShouldBindJSON
// and the subsequent handlers also need to use ShouldBindBodyWithJSON instead of ShouldBindJSON
func IdTokenVerifier() gin.HandlerFunc {
	return func(c *gin.Context) {
		var in idTokenInput

		if err := c.ShouldBindBodyWithJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if in.IdToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id_token is required"})
			c.Abort()
			return
		}

		jwt, err := base64.StdEncoding.DecodeString(in.IdToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id_token"})
			c.Abort()
			return
		}

		c.Next()
	}
}