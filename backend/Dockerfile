# Указываем базовый образ для Go
FROM golang:1.22-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./

# Устанавливаем зависимости
RUN go mod tidy

# Копируем исходный код
COPY . .

# Собираем бинарный файл
RUN go build -o main .

# Запуск контейнера
CMD ["./main"]
