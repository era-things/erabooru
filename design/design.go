package design

import (
	. "goa.design/goa/v3/dsl"
)

var _ = API("erabooru", func() {
	Title("Erabooru API")
	Server("local", func() {
		Host("localhost", func() {
			URI("http://localhost:8080")
		})
	})
})

var MediaItem = Type("MediaItem", func() {
	Attribute("id", String)
	Attribute("url", String)
	Attribute("width", Int)
	Attribute("height", Int)
	Attribute("format", String)
})

var MediaList = Type("MediaList", func() {
	Attribute("media", ArrayOf(MediaItem))
	Attribute("total", Int)
	Required("media", "total")
})

var MediaDetail = Type("MediaDetail", func() {
	Attribute("id", String)
	Attribute("url", String)
	Attribute("width", Int)
	Attribute("height", Int)
	Attribute("format", String)
	Attribute("duration", Int, "Video duration in seconds")
	Attribute("size", Int64)
	Attribute("tags", ArrayOf(String))
})

var UploadURLResponse = Type("UploadURLResponse", func() {
	Attribute("url", String)
	Attribute("object", String)
})

var _ = Service("media", func() {
	Description("Media operations")

	Method("list", func() {
		Description("List media items")
		Payload(func() {
			Attribute("q", String)
			Attribute("page", Int)
			Attribute("page_size", Int)
		})
		Result(MediaList)
		HTTP(func() {
			GET("/api/media")
			Param("q")
			Param("page")
			Param("page_size")
		})
	})

	Method("previews", func() {
		Description("List media previews")
		Payload(func() {
			Attribute("q", String)
			Attribute("page", Int)
			Attribute("page_size", Int)
		})
		Result(MediaList)
		HTTP(func() {
			GET("/api/media/previews")
			Param("q")
			Param("page")
			Param("page_size")
		})
	})

	Method("get", func() {
		Description("Get media item by ID")
		Payload(func() {
			Attribute("id", String)
			Required("id")
		})
		Result(MediaDetail)
		HTTP(func() {
			GET("/api/media/{id}")
			Param("id")
		})
	})

	Method("uploadURL", func() {
		Description("Get a presigned upload URL")
		Payload(func() {
			Attribute("filename", String)
			Required("filename")
		})
		Result(UploadURLResponse)
		HTTP(func() {
			POST("/api/media/upload-url")
			Body(func() {
				Attribute("filename")
			})
		})
	})

	Method("updateTags", func() {
		Description("Update tags for media item")
		Payload(func() {
			Attribute("id", String)
			Attribute("tags", ArrayOf(String))
			Required("id", "tags")
		})
		Result(Empty)
		HTTP(func() {
			POST("/api/media/{id}/tags")
			Param("id")
			Body(func() {
				Attribute("tags")
			})
		})
	})

	Method("delete", func() {
		Description("Delete media item")
		Payload(func() {
			Attribute("id", String)
			Required("id")
		})
		Result(Empty)
		HTTP(func() {
			DELETE("/api/media/{id}")
			Param("id")
		})
	})
})

var _ = Service("admin", func() {
	Description("Admin operations")

	Method("regenerate", func() {
		Description("Regenerate search index and metadata")
		Result(Empty)
		HTTP(func() {
			POST("/api/admin/regenerate")
		})
	})
})

var _ = Service("health", func() {
	Description("Service health check")

	Method("check", func() {
		Result(Empty)
		HTTP(func() {
			GET("/health")
		})
	})
})
