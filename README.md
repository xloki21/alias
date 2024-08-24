## Сервис URLShortener

### Запуск c помощью Docker-Compose
```docker compose --profile mongodb up```

### Создание алиас-линка на орининальный линк

```
POST http://localhost:8080/api/v1/alias
{
    "url": "http://www.ya.ru"
}
```
С помощью опционального query-параметра maxUsageCount можно установить количество переходов по сгенерированной ссылке.  
Вывод в консоль с дефолтными параметрами логгирования:
```
2024-08-25 00:51:36     info    listening on localhost:8080
2024-08-25 00:51:43     info    http    {"request": "POST", "uri": "/api/v1/alias"}
2024-08-25 00:51:43     info    service {"name": "AliasService", "fn": "AliasService::CreateOne", "origin": "http://www.ya.ru"}
2024-08-25 00:51:43     info    repo    {"name": "AliasRepository", "fn": "mongodb::SaveOne", "alias": "http://localhost:8080/Hts7D-cxEQ==", "origin": "http://www.ya.ru"}
2024-08-25 00:51:43     info    http    {"request.completed": "1ms"}

```

Варианты ответов:
```
201 - шорт-линк подготовлен. В теле ответа возвращается шорт-линк
400 - переданный запрос некорректен
500 - все остальные ошибки
```

### Создание алиас-линков набором

```
POST http://localhost:8080/api/v1/aliases
{
    "urls": ["http://www.ya.ru", ... "http://www.ya1.ru"]
}
```
Вывод в консоль:
```
2024-08-25 00:52:38     info    http    {"request": "POST", "uri": "/api/v1/aliases?maxUsageCount=3"}
2024-08-25 00:52:38     info    service {"name": "AliasService", "fn": "AliasService::CreateMany", "aliases count": 2184}
2024-08-25 00:52:38     info    repo    {"name": "AliasRepository", "fn": "mongodb::SaveMany", "aliases count": 2184}
2024-08-25 00:52:38     info    http    {"request.completed": "24ms"}

```
Варианты ответов
```
201 - шорт-линк подготовлен. В теле ответа возвращается список шорт-линков
400 - переданный запрос некорректен
500 - все остальные ошибки
```

### Удаление шорт-линка
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

### Редирект по сокращенной ссылке
```
GET http://localhost:8080/pfemZ9bl5w==
```

Варианты ответов
```
308 - Редирект
410 - Количество переходов по ссылке превысило лимит. Ссылка неактивна.
```