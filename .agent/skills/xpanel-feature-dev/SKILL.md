---
name: xpanel-feature-dev
description: Standardized workflow for developing new features, pages, and API endpoints in X-Panel.
---

# X-Panel Feature Development Skill

This skill guides you through the process of adding new features to X-Panel, ensuring consistency with the project's architecture (Gin + Vue + Go Templates).

## Architecture Overview

- **Controller** (`web/controller/`): Handles HTTP requests, parameter parsing, and response formatting.
- **Service** (`web/service/`): Contains business logic.
- **View** (`web/html/`): HTML templates (Go Templates).
- **Assets** (`web/assets/`): Static files (JS, CSS).
- **Routing**: Defined in `web/web.go` or specific controller initialization.

## Development Workflow

### 1. Create Controller

Create a new file `web/controller/<feature_name>.go`.

```go
package controller

import (
    "github.com/gin-gonic/gin"
    "x-ui/web/service"
)

type FeatureController struct {
    BaseController
    featureService service.FeatureService // Inject service if needed
}

func NewFeatureController(g *gin.RouterGroup) *FeatureController {
    a := &FeatureController{}
    a.initRouter(g)
    return a
}

func (a *FeatureController) initRouter(g *gin.RouterGroup) {
    g = g.Group("/feature")
    g.GET("/", a.index)
    // Add more routes
}

func (a *FeatureController) index(c *gin.Context) {
    if c.Request.Method == "GET" {
        c.HTML(200, "feature.html", gin.H{
            "title": "My Feature",
        })
    }
}
```

### 2. Create Service (Optional)

If the feature involves complex logic or database operations, create `web/service/<feature_name>.go`.

```go
package service

import "x-ui/database/model"

type FeatureService struct {}

func (s *FeatureService) SomeMethod() error {
    // Business logic
    return nil
}
```

### 3. Create View

Create `web/html/<feature_name>.html`. Use the standard layout.

```html
{{define "feature.html"}}
<!DOCTYPE html>
<html lang="{{.CurLang}}">
<head>
    {{template "head" .}}
    <title>{{.title}} - {{.I18n.AppTitle}}</title>
</head>
<body>
    <a-layout id="app" v-cloak>
        {{template "menu" .}}
        <a-layout-content>
            <!-- Content Here -->
        </a-layout-content>
    </a-layout>
    {{template "scripts" .}}
    <script src="assets/js/feature.js"></script>
</body>
</html>
{{end}}
```

### 4. Create Static Assets

Create `web/assets/js/<feature_name>.js` for Vue.js logic.

### 5. Register in Web Server

Modify `web/web.go` (or the main server setup) to initialize the new controller.
Look for where `New...Controller` is called.

### 6. Verify

1.  Run `go build ./...`
2.  Run `go run main.go`
3.  Access `http://localhost:<port>/feature`

## Coding Standards
- Use `x-ui/logger` for logging.
- Use `x-ui/web/json` for API responses.
- Ensure all user-facing text is i18n compatible (`web/translation/`).
