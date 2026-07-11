package httpapi

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>云朵平台 Go Demo</title>
  <style>
    :root { color-scheme: light; --bg: #f7f9fc; --panel: #ffffff; --text: #172033; --muted: #62708a; --line: #d9e1ee; --primary: #1769e0; --primary-hover: #1157bd; --accent: #15a36d; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: var(--text); background: linear-gradient(180deg, rgba(23,105,224,.08), rgba(21,163,109,.06)), var(--bg); display: flex; align-items: center; justify-content: center; padding: 32px 16px; }
    main { width: min(720px, 100%); background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 40px; box-shadow: 0 18px 50px rgba(23,32,51,.08); }
    .eyebrow { margin: 0 0 12px; color: var(--accent); font-size: 14px; font-weight: 700; }
    h1 { margin: 0; font-size: 34px; line-height: 1.2; font-weight: 750; }
    p { margin: 18px 0 0; color: var(--muted); font-size: 17px; line-height: 1.7; }
    .actions { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 28px; }
    a { display: inline-flex; align-items: center; justify-content: center; min-height: 44px; padding: 0 18px; border-radius: 6px; font-weight: 700; text-decoration: none; }
    .primary { color: #ffffff; background: var(--primary); }
    .primary:hover { background: var(--primary-hover); }
    .secondary { color: var(--text); border: 1px solid var(--line); background: #ffffff; }
    .meta { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 32px; }
    .meta div { border: 1px solid var(--line); border-radius: 6px; padding: 14px; color: var(--muted); font-size: 14px; }
    .meta strong { display: block; margin-bottom: 4px; color: var(--text); font-size: 15px; }
    @media (max-width: 640px) { main { padding: 28px 20px; } h1 { font-size: 28px; } .meta { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
<main>
  <p class="eyebrow">Cloud Deploy Go Demo</p>
  <h1>云朵平台 Go HTTP Demo</h1>
  <p>
    这是用于云朵一键部署平台的 Go 示例应用，包含标准 CRUD 接口，
    并接入 MySQL、Swagger/OpenAPI、Prometheus 和 OpenTelemetry。
  </p>
  <div class="actions">
    <a class="primary" href="/swagger-ui.html">打开 Swagger</a>
    <a class="secondary" href="/api/items">查看 Items</a>
    <a class="secondary" href="/actuator/health">健康检查</a>
  </div>
  <section class="meta">
    <div><strong>Runtime</strong>Go / net/http</div>
    <div><strong>Database</strong>MySQL compatible</div>
    <div><strong>Observability</strong>Prometheus + OTEL</div>
  </section>
</main>
</body>
</html>`

const swaggerHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Cloud Deploy Go Demo API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/v3/api-docs", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`

const openAPIJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Cloud Deploy Go Demo",
    "description": "Go CRUD demo for cloud one-click deployment",
    "version": "0.1.0"
  },
  "paths": {
    "/api/items": {
      "get": {
        "tags": ["Items"],
        "summary": "List items",
        "parameters": [
          {"name": "page", "in": "query", "schema": {"type": "integer", "default": 0, "minimum": 0}},
          {"name": "size", "in": "query", "schema": {"type": "integer", "default": 20, "minimum": 1, "maximum": 100}},
          {"name": "sort", "in": "query", "schema": {"type": "string", "default": "id"}}
        ],
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/PageResponse"}}}}}
      },
      "post": {
        "tags": ["Items"],
        "summary": "Create item",
        "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ItemRequest"}}}},
        "responses": {"201": {"description": "Created", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ItemResponse"}}}}}
      }
    },
    "/api/items/{id}": {
      "get": {
        "tags": ["Items"],
        "summary": "Get item",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer", "format": "int64"}}],
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ItemResponse"}}}}, "404": {"description": "Not Found"}}
      },
      "put": {
        "tags": ["Items"],
        "summary": "Update item",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer", "format": "int64"}}],
        "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ItemRequest"}}}},
        "responses": {"200": {"description": "OK", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ItemResponse"}}}}, "404": {"description": "Not Found"}}
      },
      "delete": {
        "tags": ["Items"],
        "summary": "Delete item",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer", "format": "int64"}}],
        "responses": {"204": {"description": "No Content"}, "404": {"description": "Not Found"}}
      }
    },
    "/actuator/health": {
      "get": {
        "tags": ["Actuator"],
        "summary": "Health check",
        "responses": {"200": {"description": "OK"}}
      }
    }
  },
  "components": {
    "schemas": {
      "ItemRequest": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": {"type": "string", "maxLength": 80},
          "description": {"type": "string", "nullable": true, "maxLength": 500}
        }
      },
      "ItemResponse": {
        "type": "object",
        "properties": {
          "id": {"type": "integer", "format": "int64"},
          "name": {"type": "string"},
          "description": {"type": "string", "nullable": true},
          "createdAt": {"type": "string", "format": "date-time"},
          "updatedAt": {"type": "string", "format": "date-time"}
        }
      },
      "PageResponse": {
        "type": "object",
        "properties": {
          "content": {"type": "array", "items": {"$ref": "#/components/schemas/ItemResponse"}},
          "page": {"type": "integer"},
          "size": {"type": "integer"},
          "totalElements": {"type": "integer", "format": "int64"},
          "totalPages": {"type": "integer"},
          "first": {"type": "boolean"},
          "last": {"type": "boolean"}
        }
      }
    }
  }
}`
