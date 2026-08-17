# API Reference

All request and response bodies are JSON. Protected endpoints require `Authorization: Bearer <access_token>`. Pagination uses `limit` (default 20, maximum 100) and `offset` (default 0).

## Endpoints

| Method | Path | Authentication | Purpose |
|---|---|---|---|
| GET | `/healthz` | No | Database-backed readiness check |
| POST | `/v1/auth/register` | No | Create an author account |
| POST | `/v1/auth/login` | No | Issue an access token |
| GET | `/v1/posts` | No | List published posts |
| GET | `/v1/posts/{slug}` | No | Read a published post |
| POST | `/v1/posts` | Yes | Create a draft, published, or archived post |
| PUT | `/v1/posts/{postID}` | Owner/admin | Replace editable post fields |
| DELETE | `/v1/posts/{postID}` | Owner/admin | Delete a post |
| GET | `/v1/posts/{postID}/comments` | No | List visible comments |
| POST | `/v1/posts/{postID}/comments` | Yes | Add a comment |
| DELETE | `/v1/comments/{commentID}` | Owner/admin | Delete a comment |

Registration accepts `email`, `display_name`, and `password`. Login accepts `email` and `password`.

Post writes accept:

```json
{
  "title": "A clear title",
  "excerpt": "Short summary",
  "content": "Article body",
  "status": "published",
  "category_id": null,
  "tags": ["Go", "Backend"]
}
```

Comment writes accept `{ "body": "Comment text" }`.

Errors have one stable shape:

```json
{"error":{"code":"invalid_input","message":"invalid_input"}}
```

Possible codes are `invalid_input`, `unauthorized`, `forbidden`, `not_found`, `conflict`, and `internal_error`.
