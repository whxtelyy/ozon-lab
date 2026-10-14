# Результаты тестирования

В ДЗ3 проверяется работа сервисов при конкурентных запросах. Для корзины используется `sync.RWMutex`, а получение информации о товарах выполняется параллельно.

Исходная версия корзины уже была защищена `RWMutex`, поэтому обнаруженной гонки в ней нет. Чтобы показать необходимость блокировок, отдельно проверялась временная копия проекта с удаленными вызовами mutex. Эта копия не входит в рабочий код.

## Проверка на гонки

Для сравнения использовались две временные копии одной версии проекта:

- `protected` - рабочий вариант с `sync.RWMutex`;
- `baseline` - тот же код, но без вызовов `Lock`, `Unlock`, `RLock` и `RUnlock` в `internal/cart/service.go`.

Тесты и остальная бизнес-логика не менялись. Проверка проводилась в Ubuntu/WSL с Go 1.23.12:

```bash
go test -race -count=1 -run '^TestConcurrentAdds$' ./internal/cart
```

В `baseline` race detector обнаружил конфликт между чтением и записью в `map`:

```text
WARNING: DATA RACE
Read at 0x00c0002733b0 by goroutine 16:
  runtime.mapaccess2_fast64()
  dz2/internal/cart.(*Service).Add()
      /tmp/ozon-sync-KiQ8IZu7/baseline/internal/cart/service.go:80 +0x2be

Previous write at 0x00c0002733b0 by goroutine 9:
  runtime.mapassign_fast64()
  dz2/internal/cart.(*Service).Add()
      /tmp/ozon-sync-KiQ8IZu7/baseline/internal/cart/service.go:83 +0x31e
```

Тест завершился с кодом `1`. В `protected` этот же тест прошел без предупреждений: `ok dz2/internal/cart 1.022s`.

**Важно:** это демонстрация гонки при отключении синхронизации, а не подтверждение ошибки в исходной рабочей версии. Замеры проводились до разделения проекта на `dz2` и `dz3`, поэтому в старом выводе осталось имя модуля `dz2`.

## Влияние RWMutex на производительность

В обеих копиях запускались одинаковые бенчмарки без сетевых запросов. Информацию о товарах возвращал один и тот же fake.

- `BenchmarkAddToCart` - добавление одного SKU в заранее созданную корзину;
- `BenchmarkGetCart` - получение корзины с двумя заранее добавленными SKU.

Подготовка данных выполнялась до `b.ResetTimer()`. Условия: Ubuntu/WSL, Ryzen 5 5600H, Go 1.23.12, `-cpu=1`, пять запусков по 300 мс.

```bash
go test ./internal/cart -run '^$' -bench 'Benchmark(AddToCart|GetCart)$' -benchmem -benchtime=300ms -count=5 -cpu=1
```

| Бенчмарк | Без блокировки, ns/op | С RWMutex, ns/op | Изменение медианы | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| AddToCart | 66.47–67.92 (медиана 66.91) | 78.58–81.93 (медиана 80.81) | +20.8% | 1 в обоих вариантах |
| GetCart | 2044–2067 (медиана 2055) | 2045–2163 (медиана 2145) | +4.4% | 19 в обоих вариантах |

С `RWMutex` медианное время `AddToCart` увеличилось на 20.8%, а `GetCart` - на 4.4%. У `GetCart` диапазоны результатов пересекаются, поэтому по этим замерам нельзя точно определить стоимость короткой блокировки чтения. `BenchmarkGetProduct` здесь не сравнивался: он не использует mutex корзины.

Это результаты последовательных бенчмарков, а не измерение пропускной способности при большой конкурентной нагрузке.

## Как повторить проверку

Команды выполняются из `dz3/` в Ubuntu/WSL. Они создают две копии во временном каталоге, не изменяя рабочие исходники:

```bash
export GOTOOLCHAIN=go1.23.12
work=$(mktemp -d)
cp -a . "$work/protected"
cp -a "$work/protected" "$work/baseline"
sed -i '/s\.mu\.Lock()/d; /s\.mu\.RLock()/d; /s\.mu\.RUnlock()/d; /defer s\.mu\.Unlock()/d' "$work/baseline/internal/cart/service.go"
```

Выполнить race-тест для обеих копий:

```bash
(cd "$work/baseline" && go test -race -count=1 -run '^TestConcurrentAdds$' ./internal/cart)
(cd "$work/protected" && go test -race -count=1 -run '^TestConcurrentAdds$' ./internal/cart)
```

Затем сравнить бенчмарки:

```bash
for version in baseline protected; do
  echo "== $version =="
  (cd "$work/$version" && go test ./internal/cart -run '^$' -bench 'Benchmark(AddToCart|GetCart)$' -benchmem -benchtime=300ms -count=5 -cpu=1)
done
```

Бенчмарки небезопасной копии запускаются без `-race` и с `-cpu=1`. После завершения можно удалить временный каталог, путь к которому находится в переменной `work`.

## Другие замеры

Ранее сравнивались две версии `GetCart` с разной логикой: старая не запрашивала сведения о товарах, новая выполняла два запроса. `RWMutex` использовался в обеих версиях.

Один запуск на Windows/amd64, Ryzen 5 5600H, Go 1.27.1:

| Бенчмарк | До, ns/op | До, allocs/op | После, ns/op | После, allocs/op |
| --- | ---: | ---: | ---: | ---: |
| GetProduct | 32.99 | 1 | 36.26 | 1 |
| AddToCart | 548.2 | 3 | 1011 | 3 |
| GetCart | 183.1 | 4 | 4837 | 17 |

Отдельный контрольный запуск `make bench` на Ubuntu/WSL с Go 1.26.0 дал следующие значения:

| Бенчмарк | ns/op | allocs/op |
| --- | ---: | ---: |
| GetProduct | 50.82 | 1 |
| AddToCart | 710.6 | 3 |
| GetCart | 3796 | 19 |

Эти результаты приведены для истории измерений. Они не подходят для оценки стоимости `RWMutex`: между замерами менялись логика `GetCart`, условия бенчмарков или среда выполнения.
