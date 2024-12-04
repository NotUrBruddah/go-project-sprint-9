package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и
// отправляет их в канал ch. При этом после записи в канал для каждого числа
// вызывается функция fn. Она служит для подсчёта количества и суммы
// сгенерированных чисел.
func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	var g int64
	defer close(ch)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			g++
			ch <- g
			fn(g)
		}
	}
}

// Worker читает число из канала in и пишет его в канал out.

// я бы описал как код в комментарии ниже через for ... range
//-------------------------------
//func Worker(in <-chan int64, out chan<- int64) {
//2. Функция Worker
//	defer close(out)
//	for v := range in {
//		out <- v
//		time.Sleep(1 * time.Millisecond)
//	}
//	return
//}
//-------------------------------
//однако в файле README  указана рекомендация делать через бесконечный цикл и оператор v, ok := <-in
//решение ниже

func Worker(in <-chan int64, out chan<- int64) {
	// 2. Функция Worker
	defer close(out)
	for {
		v, ok := <-in
		if !ok {
			break
		}
		out <- v
		time.Sleep(1 * time.Millisecond)
	}
	return
}

func main() {
	chIn := make(chan int64)

	// 3. Создание контекста
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// для проверки будем считать количество и сумму отправленных чисел
	var inputSum int64   // сумма сгенерированных чисел
	var inputCount int64 // количество сгенерированных чисел

	// генерируем числа, считая параллельно их количество и сумму
	// 6. Сделаем потокобезопасной анонимную функцию.
	// Хотя код ниже в комментарии при условии запуска одной гоурутины не потребует ни мьютексов, ни атомарных операций
	//---------------------------------------------
	//go Generator(ctx, chIn, func(i int64) {
	//	inputSum += i
	//	inputCount++
	//})
	//---------------------------------------------
	//воспользуемся атомарными операциями
	go Generator(ctx, chIn, func(i int64) {
		atomic.AddInt64(&inputSum, i)
		atomic.AddInt64(&inputCount, 1)
	})

	const NumOut = 25 // количество обрабатывающих горутин и каналов
	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		// создаём каналы и для каждого из них вызываем горутину Worker
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut)
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	// 4. Собираем числа из каналов outs
	for j, v := range outs {
		wg.Add(1)
		go func(in <-chan int64, i int64) {
			defer wg.Done()
			for val := range in {
				amounts[i]++
				chOut <- val
			}
		}(v, int64(j))
	}

	go func() {
		// ждём завершения работы всех горутин для outs
		wg.Wait()
		// закрываем результирующий канал
		close(chOut)
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	// 5. Читаем числа из результирующего канала
	for v := range chOut {
		count++
		sum += v
	}

	fmt.Println("Количество чисел", inputCount, count)
	fmt.Println("Сумма чисел", inputSum, sum)
	fmt.Println("Разбивка по каналам", amounts)

	// проверка результатов
	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}
	for _, v := range amounts {
		inputCount -= v
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}

// в результате запуска на 5 10 15 25 горутинах
//#$ go run precode.go
//Количество чисел 4588 4588
//Сумма чисел 10527166 10527166
//Разбивка по каналам [918 918 917 917 918]
//#$ go run precode.go
//Количество чисел 9163 9163
//Сумма чисел 41984866 41984866
//Разбивка по каналам [916 916 918 917 916 917 918 915 915 915]
//#$ go run precode.go
//Количество чисел 13700 13700
//Сумма чисел 93851850 93851850
//Разбивка по каналам [914 913 914 913 913 914 913 913 913 913 914 913 913 913 914]
//#$ go run precode.go
//Количество чисел 22626 22626
//Сумма чисел 255979251 255979251
//Разбивка по каналам [905 905 905 905 905 906 906 905 906 905 906 904 904 904 905 905 904 905 905 906 905 905 905 905 905]
