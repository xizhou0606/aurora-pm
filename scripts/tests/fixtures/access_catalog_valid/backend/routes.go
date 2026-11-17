package fixture

func mountThings(scoped *gin.RouterGroup) {
	scoped.GET("/things", listThings)
}
