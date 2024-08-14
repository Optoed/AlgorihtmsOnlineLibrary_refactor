## Database Schema

### `public.algorithms`
Stores algorithm information.
- **id**: Integer, Primary Key, Auto-increment.
- **title**: String, Maximum length 100, Not Null.
- **description**: Text.
- **code**: Text.
- **user_id**: Integer, Foreign Key referencing `public.users(id)`.
- **category_id**: Integer, Foreign Key referencing `public.categories(id)`.
- **rating**: Double Precision, Default 0.
- **approved**: Boolean, Default false.
- **created_at**: Timestamp, Default Current Timestamp.
- **topic**: String, Maximum length 100.
- **programming_language**: String, Maximum length 50.

### `public.categories`
Stores categories for algorithms.
- **id**: Integer, Primary Key, Auto-increment.
- **name**: String, Maximum length 50, Not Null.

### `public.email_verification_tokens`
Stores tokens for email verification.
- **user_id**: Integer, Foreign Key referencing `public.users(id)`.
- **token**: String, Maximum length 32, Primary Key.
- **created_at**: Timestamp with Time Zone, Default Now().
- **email**: String, Maximum length 255, Not Null.
- **username**: String, Maximum length 255, Not Null.

### `public.password_reset_tokens`
Stores tokens for password reset.
- **user_id**: Integer, Foreign Key referencing `public.users(id)`, Not Null.
- **token**: Text, Not Null.
- **created_at**: Timestamp, Not Null.
- **email**: String, Maximum length 255, Not Null.
- **username**: String, Maximum length 255, Not Null.

### `public.users`
Stores user information.
- **id**: Integer, Primary Key, Auto-increment.
- **username**: String, Maximum length 50, Not Null, Unique.
- **password_hash**: Text, Not Null.
- **email**: String, Maximum length 100, Not Null.
- **role**: String, Maximum length 10, Default 'user'.
- **confirmed**: Boolean, Default false.
