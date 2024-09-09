## Сервис URLShortener

### Запуск c помощью Docker-Compose
```docker compose --profile mongodb up```

### Создание алиасов

```
POST http://localhost:8080/api/v1/alias
{
    "urls": ["http://www.ya.ru", ...]
}
```
С помощью опционального query-параметра maxUsageCount можно установить количество переходов по сгенерированной ссылке.  
Вывод в консоль с дефолтными параметрами логгирования:
```
2024-09-10 00:45:19     info    http    {"request": "POST", "uri": "/api/v1/alias?maxUsageCount=3"}
2024-09-10 00:45:19     info    service {"name": "AliasService", "fn": "AliasService::CreateMany", "aliases count": 2184}
2024-09-10 00:45:19     info    repo    {"name": "AliasRepository", "fn": "mongodb::SaveMany", "aliases count": 2184}
2024-09-10 00:45:19     info    http    {"request.completed": "31ms"}


```

Варианты ответов:
```
201 - шорт-линк подготовлен. В теле ответа возвращается шорт-линк
400 - переданный запрос некорректен
500 - все остальные ошибки
```

### Удаление алиаса
```
DELETE http://localhost:8080/api/v1/remove
{
  "url": "http://localhost:8080/pfemZ9bl5w=="
}
```

Варианты ответов
```
204 - alias-линк удалён
400 - Ошибка в запросе
404 - Запрошенный шорт-линк не найден
500 - Все остальные ошибки
```

### Переход по сокращенной ссылке
```
GET http://localhost:8080/pfemZ9bl5w==
```

Варианты ответов
```
308 - Редирект
410 - Количество переходов по ссылке превысило лимит. Ссылка неактивна.
```