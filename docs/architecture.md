# System Architecture

## Design goals

The backend favors explicit dependency direction, testable business logic, and replaceable infrastructure. It is a modular monolith: one deployable process and database, with internal boundaries that can later become services only when scale or team ownership justifies that cost.

```text
HTTP client
    |
    v
Transport (routing, JSON, auth middleware)
    |
    v
Application services (validation, workflows, authorization)
    |
    v
Domain repository interfaces
    ^
    |
PostgreSQL adapters ----> PostgreSQL
```

Dependencies point inward. The domain package imports no application or infrastructure package. Services depend on domain interfaces. PostgreSQL and HTTP are outer adapters. `cmd/api` is the composition root and is the only place that constructs the full object graph.

## Responsibilities

**Domain:** Defines `User`, `Post`, `Tag`, `Category`, and `Comment`, plus repository contracts and stable errors. It contains no HTTP or SQL concepts.

**Services:** Own use cases: registration, login, token verification, post lifecycle, slug generation, tag normalization, pagination bounds, and ownership/admin authorization.

**Repositories:** Translate domain operations to parameterized PostgreSQL queries. Multi-table writes use transactions. Database-specific errors are mapped to domain errors.

**Transport:** Maps HTTP to service calls, limits and decodes input, emits consistent JSON, and extracts authenticated identity. It does not contain persistence decisions.

**Composition root:** Loads validated configuration, opens infrastructure, connects implementations to interfaces, starts HTTP, and performs graceful shutdown.

## Core flows

### Publish a post

1. Authentication middleware verifies the bearer token and attaches immutable actor claims.
2. The handler parses the request and passes an actor ID plus `PostInput` to `Posts.Create`.
3. The service validates content, creates a stable UUID/slug, and sets `published_at` only for published posts.
4. The repository inserts the post, upserts tags, and creates tag relationships in one transaction.

### Read the feed

1. Public `GET /v1/posts` reaches `Posts.List` with private content disabled.
2. Pagination is bounded to 100 records.
3. PostgreSQL uses the partial published-feed index and returns posts in reverse publication order.

## Data model

- A user owns many posts and comments.
- A post optionally belongs to one category.
- Posts and tags have a many-to-many relationship through `post_tags`.
- Deleting a user cascades their content; deleting a category preserves posts and clears the association.
- Post status is `draft`, `published`, or `archived`. A published post must have `published_at`.

## Scaling path

Start with vertical API/database scaling and PostgreSQL read indexes. Add cursor pagination before feeds become large. Add Redis only for measured cache or rate-limit needs. Store uploaded media in object storage rather than PostgreSQL. Background jobs should use an outbox table so database state and emitted work cannot diverge. Read replicas and search indexing can be added behind new repository adapters without changing service rules.

## Intentional boundaries

This first version omits refresh tokens, email verification, password reset, media upload, category administration, full-text search, and an outbox worker. Those are separate workflows rather than placeholders inside core request paths. The schema already supports categories; an administration API can be added without changing post ownership semantics.
