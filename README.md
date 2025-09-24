
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

пагинация по постам 



### Выбор хранилища
Хранилище задается через переменные среды, переменные среды лежат в корне проекта в .env файле
если не указан "postgres" будет выбран по умолчанию inmemory


