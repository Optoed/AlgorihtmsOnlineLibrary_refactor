# AlgorithmsOnlineLibrary

AlgorithmsOnlineLibrary is a web application for creating and storing algorithms.

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Installation](#installation)
- [Usage](#usage)
- [Dependencies](#dependencies)
- [API Endpoints](#api-endpoints)
- [Database Schema](#database-schema)
- [License](#license)

## Features

- **User authentication using JWT.**
- **Password hashing with bcrypt** for security.
- **Role-based access control** (e.g., Admin, User).
- **Algorithm submission and management.**
- **Email notifications** for user-related actions.
- **RESTful API design** using Gorilla Mux.
- **CORS support** for cross-origin requests.

## Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/yourusername/yourproject.git
   cd yourproject

2. **Install Dependencies:**
   ```bash
   go mod tidy

3. **Set Up the Environment:**

    Create a .env file in the root of your project and
add the necessary environment variables (e.g., database connection string, JWT secret key).
   ```bash
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=yourusername
   DB_PASSWORD=yourpassword
   DB_NAME=yourdbname
   JWT_SECRET=my_secret_key
   EMAIL_HOST=smtp.example.com
   EMAIL_PORT=465
   EMAIL_USER=youremail@example.com
   EMAIL_PASSWORD=yourpassword

4. **Set Up PostgreSQL:**

    Ensure you have PostgreSQL installed and running. Create the database and tables as described in the schema below.
   ```bash
   psql -U yourusername -d yourdbname -a -f setup.sql

5. **Install Frontend Dependencies:**

    Navigate to the frontend directory and install the dependencies.
    ```bash
    cd frontend
    npm install

6. **Start the Development Servers:**

- **Backend:**
  ```bash
  go run main.go

- **Frontend:**
  ```bash
  npm start
  
## Usage

   Once the server is running, you can interact with the API using web-client (frontend on port http://localhost:3000) and tools like curl or Postman.
The base URL is typically http://localhost:8080.

## Dependencies

This project uses the following dependencies:

- **Gorilla Mux**: HTTP router and URL matcher for Go.
- **JWT-Go**: JSON Web Token implementation for Go, useful for handling authentication and authorization.
- **Godotenv**: Loads environment variables from a `.env` file into Go applications, simplifying configuration management.
- **Gomail**: Package for sending emails in Go.
- **CORS**: Middleware for handling Cross-Origin Resource Sharing (CORS) in Go HTTP servers.
- **bcrypt**: Password hashing library for securely hashing and comparing passwords.
- **Axios**: Promise-based HTTP client for the frontend.

## API Endpoints

### User Authentication

- **POST /login**: User login and JWT token generation.
- **POST /register**: Register a new user.

### Algorithm Management

- **GET /algorithms**: Retrieve a list of all algorithms.
- **POST /algorithms**: Submit a new algorithm.
- **GET /algorithms/{id}**: Get details of a specific algorithm.

## Database Schema

For detailed information about the database schema, see [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md).


## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE.md) file for details.
