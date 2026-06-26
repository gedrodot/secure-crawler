# Stage 1: Сборка
FROM golang:1.25-alpine AS builder
# Рабочая директория внутри контейнера
WORKDIR /app

# Копируем зависимости и скачиваем их
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь остальной код
COPY . .

# Принимаем аргумент при сборке
ARG SERVICE_NAME

# Собираем конкретный микросервис. 
# Бинарник кладем в корень первого контейнера (/main), чтобы его было легко забрать
RUN CGO_ENABLED=0 GOOS=linux go build -o /main ./cmd/${SERVICE_NAME}

# Stage 2: Финальный легковесный образ
FROM alpine:3.18
RUN apk --no-cache add ca-certificates
WORKDIR /app

# Забираем собранный бинарник из первой стадии (builder)
COPY --from=builder /main .

# Запускаем его
CMD ["./main"]