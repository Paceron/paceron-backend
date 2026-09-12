package controllers

import "github.com/gin-gonic/gin"

// respondCatalogError responde con el shape {"message": "..."} pedido por el
// frontend para el dominio de catálogo/calendario — distinto del
// apierror.APIError SCREAMING_SNAKE del resto del backend (design.md D6 de
// catalogo-planes-entrenamiento).
func respondCatalogError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"message": message})
}
