-- Задача 1
SELECT name, age
FROM users
WHERE city = 'Москва'
  AND age = (
      SELECT MAX(age)
      FROM users
      WHERE city = 'Москва'
  )
ORDER BY name;

-- Задача 2
SELECT city, COUNT(*) AS user_count
FROM users
GROUP BY city
HAVING COUNT(*) > 1
ORDER BY user_count DESC, city;

-- Задача 3
SELECT u.name, COALESCE(SUM(o.price), 0) AS total_spent
FROM users AS u
LEFT JOIN orders AS o ON o.user_id = u.id
GROUP BY u.id, u.name
ORDER BY total_spent DESC, u.name;

-- Задача 4
SELECT DISTINCT u.name
FROM users AS u
JOIN orders AS o ON o.user_id = u.id
WHERE o.item = 'наушники'
ORDER BY u.name;

-- Задача 5
SELECT item, price
FROM orders
WHERE price > 10000
  AND price = (
      SELECT MIN(price)
      FROM orders
      WHERE price > 10000
  )
ORDER BY item, id;
