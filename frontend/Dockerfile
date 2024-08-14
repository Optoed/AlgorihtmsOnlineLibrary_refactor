# Указываем базовый образ для Node.js
FROM node:18-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем package.json и package-lock.json для установки зависимостей
COPY algorithms-online-library-frontend/package*.json ./

# Устанавливаем зависимости
RUN npm install

# Копируем весь код проекта
COPY algorithms-online-library-frontend/ ./

# Выполняем сборку проекта
RUN npm run build

# Указываем базовый образ для деплоя
FROM nginx:alpine

# Копируем сгенерированные статические файлы в папку Nginx
COPY --from=builder /app/build /usr/share/nginx/html

# Открываем порт
EXPOSE 80

# Запускаем Nginx
CMD ["nginx", "-g", "daemon off;"]
