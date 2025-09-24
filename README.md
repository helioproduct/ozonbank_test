
### Запуск
- **make up** (из корня проекта) — поднимает бд, прогоняяет миграции, стартует приложение (docker-compose)


## Makefile команды

- **make up** — запустить сервисы в фоне (если образы уже собраны).  
- **make down** — остановить все контейнеры.  
- **make clear** — остановить контейнеры и удалить тома (очистка данных БД).  
- **make rebuild** — пересобрать образы и заново поднять контейнеры.  
- **make test** — прогнать unit-тесты


### если нужен отдельный docker-образ

```bash
docker build -t myreddit -f deployments/Dockerfile .
```


### использование Docker-образа

```bash
docker run --rm -p 8080:8080 \                                                                   
  -e STORAGE_TYPE=postgres \
  -e POSTGRES_HOST=host.docker.internal \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_USER=postgresuser \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=myreddit \
  -e POSTGRES_SSLMODE=disable \
  -e HTTP_PORT=8080 \
  -e WS_KEEPALIVE=10 \
  myreddit -tail
```


### Запросы к API
после make up playground будет доступен по адресу localhost:8080 (если ничего не менялось в .env)

Полная graphql-схема лежит в [./docs](./docs/schema.graphqls)


### примеры запросов

### Создать пост
```graphql
mutation {
  createPost(title: "Пример поста", body: "Содержимое поста", userId: "1") {
    id
    title
    body
    userId
    createdAt
  }
}
```

### Добавить комментарий
```graphql
mutation {
  createComment(postId: "1", userId: "2", body: "Пример комментария") {
    id
    body
    userId
    postId
    createdAt
  }
}
```


### Добавить ответ на  комментарий
```graphql
mutation {
  createComment(postId: "1", parentId: "10", userId: "3", body: "Пример ответа") {
    id
    body
    parentId
    userId
    createdAt
  }
}
```


### Пагинация
key-set пагинация по постам, комментариям, ответам на комментарии
(на основе данной статьи https://www.apollographql.com/blog/explaining-graphql-connections)  

```graphql
input PageInput {
  limit: Int
  before: Cursor
  after: Cursor
}

type PageInfo {
  startCursor: Cursor
  endCursor: Cursor
  hasNextPage: Boolean!
  count: Int!
}
```

#### пример запроса
```graphql
query {
  posts(page: { limit: 10 }) {
    nodes {
      id
      title
      body
      userId
      createdAt
      commentsEnabled
    }
    pageInfo {
      count
      hasNextPage
      startCursor
      endCursor
    }
  }
}
```


#### ответ
```graphql
{
  "data": {
    "posts": {
      "nodes": [
        {
          "id": "5",
          "title": "Пост 3",
          "body": "Текст поста 3",
          "userId": "42",
          "createdAt": "2025-09-24T14:45:11.090943Z",
          "commentsEnabled": true
        },
        {
          "id": "4",
          "title": "Пост 2",
          "body": "Текст поста 2",
          "userId": "42",
          "createdAt": "2025-09-24T14:45:03.223335Z",
          "commentsEnabled": true
        },
      ],
      "pageInfo": {
        "count": 2,
        "hasNextPage": false,
        "startCursor": "eyJDcmVhdGVkQXQiOiIyMDI1LTA5LTI0VDE0OjQ1OjExLjA5MDk0M1oiLCJJRCI6NX0=",
        "endCursor": "eyJDcmVhdGVkQXQiOiIyMDI1LTA5LTI0VDE0OjQyOjI1LjQ1NzM5M1oiLCJJRCI6MX0="
      }
    }
  }
}
```




### Таблицы и индексы в БД
```sql
CREATE TABLE posts (
    id                  BIGSERIAL PRIMARY KEY,
    title               TEXT        NOT NULL,
    body                TEXT        NOT NULL,
    user_id             BIGINT      NOT NULL CHECK(user_id >= 0),
    comments_enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE comments (
    id         BIGSERIAL PRIMARY KEY,
    post_id    BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    parent_id  BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL,                
    body       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE INDEX idx_comments_pagination
    ON comments (post_id, parent_id, created_at, id);

CREATE INDEX idx_comments_roots
    ON comments (post_id, created_at, id)
    WHERE parent_id IS NULL;


CREATE INDEX idx_posts_pagination ON posts (created_at DESC, id DESC);
```



### Выбор хранилища
Хранилище задается через переменные среды, переменные среды лежат в корне проекта в .env файле
если не указан "postgres" будет выбран по умолчанию inmemory





### Структура проекта

```bash
── cmd
├── config
├── database
├── deployments
├── docs
├── internal
│   ├── adapter
│   │   ├── in
│   │   │   └── graphql
│   │   └── out
│   │       ├── commentbus
│   │       │   └── inmemory
│   │       └── storage
│   │           ├── inmemory
│   │           ├── postgres
│   ├── app
│   ├── model
│   └── service
├── pkg
```