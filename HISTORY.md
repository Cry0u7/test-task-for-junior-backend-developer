У меня есть задача щас добавить переодичность в апи задач, существуют ли для этого какие-нибудь правила и стандарты, если сейчас это выглядит так, что у нас есть задача с такими полями package task


import "time"


type Status string


const (
    StatusNew        Status = "new"
    StatusInProgress Status = "in_progress"
    StatusDone       Status = "done"
)


type Task struct {
    ID          int64     `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Status      Status    `json:"status"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}


func (s Status) Valid() bool {
    switch s {
    case StatusNew, StatusInProgress, StatusDone:
        return true
    default:
        return false
    }
}
Выполнено 2 шага
Да — для периодичности у задач есть де-факто стандарт: iCalendar RRULE из RFC 5545, и его часто берут как формат описания расписания даже вне календарей. Если API внутреннее и не хочется тащить весь календарный стандарт, лучше все равно проектировать модель так, чтобы она была совместима по смыслу с RRULE: частота, интервал, дни недели, ограничение по дате/количеству, часовой пояс, исключения.

Что обычно считается нормой
RFC 5545 описывает recurrence rule, где ключевой обязательный элемент — FREQ, а дальше могут идти INTERVAL, UNTIL, COUNT и другие части правила. Google Calendar использует именно такой подход: у сущности есть поле recurrence, куда передается строка вида RRULE:FREQ=WEEKLY;UNTIL=..., а отдельные инстансы серии имеют связь с родительской серией и свой идентификатор исходного времени.

На практике это означает две хорошие стратегии:

хранить одно поле recurrence_rule как RRULE-строку;

или хранить нормализованную структуру schedule, а RRULE уметь собирать/разбирать внутри сервиса.

Как лучше моделировать Task
Если задача у вас сейчас — это просто единичная запись, то для периодичности лучше не смешивать “шаблон серии” и “конкретное выполнение” в одну сущность без явного разделения. У Google recurring-сущность и ее экземпляры различаются: у экземпляра есть ссылка на родительскую серию и originalStartTime, а само правило хранится у родителя. Это хороший ориентир и для вашего API: одна запись описывает серию, а выполнения либо вычисляются на лету, либо создаются как отдельные occurrence/job records.

Я бы разделил так:

Task или TaskSeries — шаблон задачи;

TaskOccurrence — конкретный запуск/экземпляр;

Recurrence/Schedule — правило повторения.

Рекомендуемая структура
Для вашего кейса минимально адекватная модель может быть такой:

go
type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type Task struct {
	ID           int64       `json:"id"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	Status       Status      `json:"status"`
	IsRecurring  bool        `json:"is_recurring"`
	Recurrence   *Recurrence `json:"recurrence,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type Frequency string

const (
	FreqDaily   Frequency = "daily"
	FreqWeekly  Frequency = "weekly"
	FreqMonthly Frequency = "monthly"
	FreqYearly  Frequency = "yearly"
)

type Recurrence struct {
	Frequency  Frequency  `json:"frequency"`
	Interval   int        `json:"interval,omitempty"`     // каждые N дней/недель/месяцев
	ByWeekDays []string   `json:"by_week_days,omitempty"` // MO,TU,...
	Count      *int       `json:"count,omitempty"`        // всего повторений
	Until      *time.Time `json:"until,omitempty"`        // до даты
	TimeZone   string     `json:"time_zone,omitempty"`    // Europe/Moscow
	RRule      string     `json:"rrule,omitempty"`        // опционально для совместимости
}
Такой вариант проще валидировать и документировать, чем сразу заставлять клиентов руками собирать RRULE. Но если у вас есть шанс интеграции с календарями, внешними системами или импортом/экспортом расписаний, поле RRule очень полезно оставить как каноническое или производное.

Правила проектирования
Хорошие правила для API с периодичностью обычно такие:

Нельзя одновременно задавать count и бесконечную серию без until, если бизнес не готов к бесконечным сериям. RFC допускает открытые правила, но в бизнес-API это часто создает проблемы с генерацией экземпляров.

Если есть until, нужно явно фиксировать таймзону, иначе будут ошибки на DST и “переезде” времени. RFC и Google оба завязаны на корректную работу с date-time и time zone.

Нужны исключения: “каждый понедельник, кроме 1 мая”. В RFC для этого есть exception dates, то есть модель исключений — это не экзотика, а нормальная часть стандарта.

Нужна семантика изменения серии: только один экземпляр, этот и все следующие, или вся серия. В календарных системах это важный кейс; Google для “все следующие” фактически делит серию на две.

Особенно важный момент: не стоит делать только поле вроде periodicity: "weekly" без стартовой даты, таймзоны и ограничителей серии. Такой формат быстро ломается на вопросах “с какого дня?”, “до когда?”, “по каким дням недели?” и “что делать с конкретным исключением?”.

Практический совет
Если API ваше и клиентов немного, лучший баланс такой:

Внешнее API — структурированное поле recurrence.

Внутри сервиса — преобразование в RRULE-подобную модель.

Отдельная таблица/модель для task_occurrences и task_exceptions.

Если хочешь, я могу сразу предложить готовый Go-дизайн: Task, Recurrence, TaskOccurrence, JSON-примеры для POST/PUT и правила валидации.

Описание существующего проекта
Существует медицинская информационная система, включающая в себя несколько модулей.
Вам дается исходный код проекта, который представляет собой API модуля трекера задач. В нём уже реализованы возможности создания, редактирования, удаления и просмотра задач (получение задачи по ID и получение списка задач).
Через этот модуль врачи могут ставить себе рабочие задачи: провести операцию, связаться с клиентом, сделать обход пациентов и так далее.


Исходный код модуля трекера задач - https://github.com/medods/test-task-for-junior-backend-developer.

Что нужно сделать?
Задача: добавить возможность задавать настройки периодичности задачи.

Описание:
Многие задачи для персонала периодические. Кому-то необходимо выполнять ежедневный обзвон пациентов, кто-то по чётным дням занимается формированием отчетности, а кто-то проводит еженедельные инвентаризации.
Необходимо дать возможность пользователю определять периодичность задачи и сделать так, чтобы задачи в трекере создавались с учетом этих настроек.
Типы задач по периодичности:
ежедневные (каждый n-й день),
ежемесячные - задачи на каждое определенное число месяца (от 1 до 30),
на конкретные даты — задачи создаются только на указанные даты,
четные/нечетные дни месяца - задачи создаются либо только на четные числа, либо только на нечетные.


⚠️ Ваша задача — не просто написать код, а реализовать фичу, которая закроет потребности клиента.
Подумайте, как это сделать наилучшим образом. Продумайте все возможные варианты использования и реализуйте ваше решение так, как считаете нужным/правильным. Вот посмотри полно еописание задачи и скажи, что можно с этим сделать, чтобы было максимально близко к изначальной структуре и максимально чисто
Выполнено 3 шага
Да, это можно сделать чисто и близко к текущей структуре, если не пытаться сразу превращать трекер в полноценный календарь, а добавить отдельную небольшую модель периодичности к существующей задаче. Для таких вещей общая практика — хранить у “родительской” сущности правило повторения, а конкретные экземпляры либо вычислять, либо создавать на его основе; именно так устроены recurring events в календарных API.

Общий подход
По описанию задачи у вас не нужен полный RFC 5545 с RRULE, потому что поддерживаются только 4 бизнес-сценария: каждый n-й день, фиксированные числа месяца, конкретные даты, четные/нечетные дни месяца. Поэтому самое чистое решение — не вводить строковые “магические” правила, а сделать типизированную структуру recurrence с явным type и параметрами под каждый режим.

При этом архитектурно лучше мыслить так: есть шаблон задачи, а есть “факты появления задач” в конкретные даты. В recurring-моделях это стандартный подход: у серии хранится правило, а экземпляры связаны с родителем и при необходимости могут жить отдельно.

Что менять в модели
Если держаться максимально близко к вашей текущей Task, я бы не делал новую большую иерархию сущностей на первом этапе. Лучше добавить в Task 3 поля:

is_recurring bool

recurrence *Recurrence

start_date или active_from как точку отсчета для генерации.

Сама Recurrence может быть такой:

go
type RecurrenceType string

const (
	RecurrenceNone        RecurrenceType = "none"
	RecurrenceDaily       RecurrenceType = "daily"
	RecurrenceMonthlyDays RecurrenceType = "monthly_days"
	RecurrenceSpecific    RecurrenceType = "specific_dates"
	RecurrenceParity      RecurrenceType = "month_day_parity"
)

type MonthDayParity string

const (
	ParityOdd  MonthDayParity = "odd"
	ParityEven MonthDayParity = "even"
)

type Recurrence struct {
	Type          RecurrenceType  `json:"type"`
	EveryNDays    *int            `json:"every_n_days,omitempty"`     // для daily
	MonthDays     []int           `json:"month_days,omitempty"`       // 1..30
	SpecificDates []time.Time     `json:"specific_dates,omitempty"`   // только даты
	Parity        *MonthDayParity `json:"parity,omitempty"`           // odd/even
}
Это лучше, чем одно поле вроде periodicity: "monthly", потому что сразу видно допустимые настройки и легко валидировать комбинации. В recurring API важна именно ясная структура правила, а не свободный текст.

Правила валидации
Чтобы решение было “чистым”, основная логика должна быть не в хендлере, а в валидаторе и в доменном методе Matches(date) bool. Для каждого type должны быть строгие правила:

daily — обязателен every_n_days >= 1;

monthly_days — массив 1..30, без дублей;

specific_dates — непустой список дат, без времени или с нормализацией к полуночи;

month_day_parity — только odd или even.

Я бы еще добавил общее правило: если recurrence != nil, то status не должен означать состояние всей серии как “done навсегда”, потому что у периодической задачи “done” обычно относится к конкретному выполнению, а не ко всей схеме. По модели recurring events экземпляры и серия — это разные уровни состояния.

Как реализовать без ломки API
Самый безопасный путь — оставить CRUD у Task, но поменять смысл списка задач: API должно отдавать задачи, актуальные на дату, а не просто все шаблоны. У recurring API это аналог выдачи инстансов серии за диапазон дат.

То есть я бы предложил такую эволюцию:

POST /tasks — создает обычную или периодическую задачу;

GET /tasks?date=2026-04-30 — возвращает задачи, которые должны существовать на эту дату;

GET /tasks?from=2026-04-01&to=2026-04-30 — возвращает развернутые вхождения;

GET /tasks/:id — возвращает сам шаблон задачи, включая recurrence.

Это позволяет не создавать физические записи “на каждый день вперед” без необходимости. Такой ленивый расчет обычно чище и дешевле, чем фоновые генераторы, если объем пока небольшой.

Что я бы сделал в твоем случае
Если цель — “максимально близко к изначальной структуре и максимально чисто”, я бы выбрал такой вариант:

не вводить полноценный RRULE;

не плодить пока отдельную таблицу task_occurrences;

хранить recurrence прямо внутри Task;

сделать сервис recurrence.Matcher или scheduler, который умеет отвечать на вопрос: “эта задача должна быть показана/создана на дату X?”.

Тогда доменная логика будет выглядеть примерно так:

У задачи есть базовая дата начала.

У задачи может быть Recurrence.

Метод OccursOn(date time.Time) bool решает, активна ли задача в конкретный день.

Список задач фильтруется через этот метод.

Для ваших типов это особенно удобно:

daily: разница в днях от start_date, делится на every_n_days;

monthly_days: день месяца входит в список;

specific_dates: дата входит в set;

parity: day % 2 == 0/1.

Из нюансов я бы обязательно продумал:

часовой пояс, лучше считать всё в локальной бизнес-зоне;

что делать с 31 числом, у вас ТЗ уже умно ограничило 1..30;

редактирование recurrence: изменение правила меняет будущие появления, а не историю. Это соответствует распространенной модели recurring-серий.

Если хочешь, следующим сообщением я могу уже дать конкретный проектный дизайн под этот репозиторий: какие файлы примерно менять, какие структуры добавить, как будет выглядеть JSON запроса, и как написать OccursOn() без избыточной архитектуры.

Напиши тогда мне код для файла task.go

Да — ниже даю вариант task.go, который остается максимально близким к вашей текущей модели: Task почти не ломается, а периодичность добавляется отдельной вложенной структурой с валидацией. Для recurring-сущностей обычно и рекомендуют хранить правило отдельно от основной сущности, а не распихивать его по случайным полям.

go
package task

import (
	"fmt"
	"sort"
	"time"
)

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type RecurrenceType string

const (
	RecurrenceTypeDaily       RecurrenceType = "daily"
	RecurrenceTypeMonthlyDays RecurrenceType = "monthly_days"
	RecurrenceTypeDates       RecurrenceType = "dates"
	RecurrenceTypeParity      RecurrenceType = "parity"
)

type DayParity string

const (
	DayParityOdd  DayParity = "odd"
	DayParityEven DayParity = "even"
)

type Task struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      Status      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`

	Recurrence *Recurrence `json:"recurrence,omitempty"`
}

type Recurrence struct {
	Type RecurrenceType `json:"type"`

	// daily: every n-th day начиная с StartDate
	EveryNDays int `json:"every_n_days,omitempty"`

	// monthly_days: список чисел месяца от 1 до 30
	MonthDays []int `json:"month_days,omitempty"`

	// dates: задачи создаются только на указанные даты
	Dates []Date `json:"dates,omitempty"`

	// parity: odd/even дни месяца
	Parity DayParity `json:"parity,omitempty"`

	// Базовая дата для daily и в целом разумная точка начала действия правила
	StartDate Date `json:"start_date"`
}

// Date хранит только календарную дату без бизнес-смысла времени суток.
// Это удобнее для задач, которые должны возникать "на день", а не "на timestamp".
type Date struct {
	time.Time
}

func NewDate(t time.Time) Date {
	return Date{Time: normalizeDate(t)}
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

func (d *Date) UnmarshalJSON(data []byte) error {
	raw := string(data)
	if raw == `""` || raw == `null` {
		d.Time = time.Time{}
		return nil
	}

	t, err := time.Parse(`"2006-01-02"`, raw)
	if err != nil {
		return fmt.Errorf("invalid date format: expected YYYY-MM-DD")
	}

	d.Time = normalizeDate(t)
	return nil
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

func (t Task) Valid() bool {
	if t.Title == "" {
		return false
	}

	if !t.Status.Valid() {
		return false
	}

	if t.Recurrence != nil && !t.Recurrence.Valid() {
		return false
	}

	return true
}

func (r Recurrence) Valid() bool {
	if r.StartDate.Time.IsZero() {
		return false
	}

	switch r.Type {
	case RecurrenceTypeDaily:
		return r.validDaily()
	case RecurrenceTypeMonthlyDays:
		return r.validMonthlyDays()
	case RecurrenceTypeDates:
		return r.validDates()
	case RecurrenceTypeParity:
		return r.validParity()
	default:
		return false
	}
}

func (r Recurrence) OccursOn(date time.Time) bool {
	if !r.Valid() {
		return false
	}

	target := normalizeDate(date)
	start := normalizeDate(r.StartDate.Time)

	if target.Before(start) {
		return false
	}

	switch r.Type {
	case RecurrenceTypeDaily:
		diffDays := int(target.Sub(start).Hours() / 24)
		return diffDays%r.EveryNDays == 0

	case RecurrenceTypeMonthlyDays:
		day := target.Day()
		for _, d := range r.MonthDays {
			if d == day {
				return true
			}
		}
		return false

	case RecurrenceTypeDates:
		for _, d := range r.Dates {
			if sameDate(d.Time, target) {
				return true
			}
		}
		return false

	case RecurrenceTypeParity:
		day := target.Day()
		if r.Parity == DayParityEven {
			return day%2 == 0
		}
		return day%2 != 0

	default:
		return false
	}
}

func (r *Recurrence) Normalize() {
	if r == nil {
		return
	}

	r.StartDate = NewDate(r.StartDate.Time)

	switch r.Type {
	case RecurrenceTypeMonthlyDays:
		r.MonthDays = uniqueSortedDays(r.MonthDays)
	case RecurrenceTypeDates:
		r.Dates = uniqueSortedDates(r.Dates)
	}
}

func (r Recurrence) validDaily() bool {
	return r.EveryNDays > 0
}

func (r Recurrence) validMonthlyDays() bool {
	if len(r.MonthDays) == 0 {
		return false
	}

	for _, day := range r.MonthDays {
		if day < 1 || day > 30 {
			return false
		}
	}

	return true
}

func (r Recurrence) validDates() bool {
	if len(r.Dates) == 0 {
		return false
	}

	for _, d := range r.Dates {
		if d.Time.IsZero() {
			return false
		}
	}

	return true
}

func (r Recurrence) validParity() bool {
	return r.Parity == DayParityOdd || r.Parity == DayParityEven
}

func normalizeDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func sameDate(a, b time.Time) bool {
	a = normalizeDate(a)
	b = normalizeDate(b)
	return a.Equal(b)
}

func uniqueSortedDays(days []int) []int {
	seen := make(map[int]struct{}, len(days))
	result := make([]int, 0, len(days))

	for _, day := range days {
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		result = append(result, day)
	}

	sort.Ints(result)
	return result
}

func uniqueSortedDates(dates []Date) []Date {
	seen := make(map[string]struct{}, len(dates))
	result := make([]Date, 0, len(dates))

	for _, d := range dates {
		n := normalizeDate(d.Time)
		key := n.Format("2006-01-02")

		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, NewDate(n))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.Before(result[j].Time)
	})

	return result
}
Что в этом коде важно
Здесь я специально оставил Task почти таким же, как у тебя был, и добавил только Recurrence *Recurrence, чтобы изменение было минимальным и не ломало старые обычные задачи. Для JSON-дат я использовал отдельный тип Date, потому что в Go time.Time с omitempty и временем суток часто создает лишний шум, а для этой бизнес-задачи нужны именно календарные даты.

Еще важная часть — метод OccursOn(date), потому что именно он потом позволит чисто реализовать выдачу задач на конкретную дату или за диапазон, не зашивая бизнес-логику в хендлеры. Разделение “правило серии” и “проверка конкретной даты” соответствует тому, как recurring-сущности обычно моделируются в календарных системах.

Что я бы сразу подправил дальше
Я бы следующим шагом сделал:

Validate() error вместо Valid() bool, чтобы возвращать понятные ошибки;

Normalize() вызывать перед сохранением;

в service/repository добавить фильтрацию задач через OccursOn().

И еще один практический момент: для parity сейчас логика считает четность числа месяца, как и требует ТЗ, а не четность календарного дня в году. Это правильно именно под ваше описание задачи, где речь идет про “четные/нечетные числа месяца”.

Могу следующим сообщением сразу написать и улучшенную версию с Validate() error, чтобы ее можно было почти без стыда вставлять в тестовое задание.

Я не хочу ставить срок выполнения

Тогда это ок — по ТЗ ты имеешь полное право не вводить срок выполнения, если явно объяснишь своё решение.

Сейчас по фидбеку у тебя две задачи:

Объяснить, почему поля нет.

Сделать тексты ошибок конкретнее.

1. Что написать, чтобы “отбить” срок выполнения
Добавь в README.md короткий блок “Предположения и упрощения”, где явно зафиксируй:

text
### Поле "Срок выполнения"

В концептуальном макете формы есть поле "Срок выполнения". В текстовом описании задания оно не фигурирует, не описано поведение и сценарии использования.

В рамках тестового задания я сфокусировался на реализации логики периодичности (ежедневные, ежемесячные, по датам, по четности), а поле "Срок выполнения" сознательно не реализовывал, чтобы не придумывать за продукт и не смешивать дедлайн с правилом повторения.
Если хочешь чуть жёстче/профессиональнее:

что “дедлайн” (due date) — отдельная бизнес-сущность, которую лучше проектировать отдельно от периодичности;

без явного бизнес-сценария (истёк срок → что делаем?) это поле может ввести в заблуждение и выглядеть “мертвым”.

Так ты покажешь, что это не забывчивость, а осознанное решение.

2. Как поправить сообщения об ошибках
Это уже прямое замечание по коду, его лучше исправить.

Сделай так:

В домене добавь Validate() с конкретными ошибками.

В usecase используй Validate() и прокидывай текст наружу.

Пример для Recurrence (минимально инвазивно к твоему коду):

go
func (r Recurrence) Validate() error {
    if r.StartDate.Time.IsZero() {
        return fmt.Errorf("recurrence.start_date is required")
    }

    switch r.Type {
    case RecurrenceTypeDaily:
        if r.EveryNDays <= 0 {
            return fmt.Errorf("recurrence.every_n_days must be > 0 for type 'daily'")
        }
        return nil

    case RecurrenceTypeMonthlyDays:
        if len(r.MonthDays) == 0 {
            return fmt.Errorf("recurrence.month_days must not be empty for type 'monthly_days'")
        }
        for _, day := range r.MonthDays {
            if day < 1 || day > 30 {
                return fmt.Errorf("recurrence.month_days value %d is out of range [1,30]", day)
            }
        }
        return nil

    case RecurrenceTypeDates:
        if len(r.Dates) == 0 {
            return fmt.Errorf("recurrence.dates must not be empty for type 'dates'")
        }
        for _, d := range r.Dates {
            if d.Time.IsZero() {
                return fmt.Errorf("recurrence.dates contains empty date")
            }
        }
        return nil

    case RecurrenceTypeParity:
        if r.Parity != DayParityOdd && r.Parity != DayParityEven {
            return fmt.Errorf("recurrence.parity must be 'odd' or 'even' for type 'parity'")
        }
        return nil

    default:
        return fmt.Errorf("recurrence.type is invalid")
    }
}

func (r Recurrence) Valid() bool {
    return r.Validate() == nil
}
И для задачи:

go
func (t Task) Validate() error {
    if strings.TrimSpace(t.Title) == "" {
        return fmt.Errorf("title is required")
    }

    if !t.Status.Valid() {
        return fmt.Errorf("status is invalid")
    }

    if t.Recurrence != nil {
        if err := t.Recurrence.Validate(); err != nil {
            return err
        }
    }

    return nil
}

func (t Task) Valid() bool {
    return t.Validate() == nil
}
В usecase (service.go) вместо:

go
if model.Recurrence != nil {
    model.Recurrence.Normalize()
}
if !model.Valid() {
    return nil, fmt.Errorf("%w: invalid task", ErrInvalidInput)
}
сделай:

go
if model.Recurrence != nil {
    model.Recurrence.Normalize()
}

if err := model.Validate(); err != nil {
    return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
}
То же самое в Update. Тогда клиент получит ошибки уровня:

"error": "invalid input: recurrence.start_date is required"

"error": "invalid input: recurrence.every_n_days must be > 0 for type 'daily'"

и претензия про “без конкретики” уйдёт.

Если хочешь, я могу следом выдать тебе полный обновлённый task.go с Validate() и точный патч для service.go, чтобы ты мог просто заменить файл.

package task


import (
    "fmt"
    "sort"
    "time"
)


type Status string


const (
    StatusNew        Status = "new"
    StatusInProgress Status = "in_progress"
    StatusDone       Status = "done"
)


type RecurrenceType string


const (
    RecurrenceTypeDaily       RecurrenceType = "daily"
    RecurrenceTypeMonthlyDays RecurrenceType = "monthly_days"
    RecurrenceTypeDates       RecurrenceType = "dates"
    RecurrenceTypeParity      RecurrenceType = "parity"
)


type DayParity string


const (
    DayParityOdd  DayParity = "odd"
    DayParityEven DayParity = "even"
)


type Task struct {
    ID          int64       `json:"id"`
    Title       string      `json:"title"`
    Description string      `json:"description"`
    Status      Status      `json:"status"`
    CreatedAt   time.Time   `json:"created_at"`
    UpdatedAt   time.Time   `json:"updated_at"`


    Recurrence *Recurrence `json:"recurrence,omitempty"`
}


type Recurrence struct {
    Type RecurrenceType `json:"type"`


    EveryNDays int `json:"every_n_days,omitempty"`


    MonthDays []int `json:"month_days,omitempty"`


    Dates []Date `json:"dates,omitempty"`


    Parity DayParity `json:"parity,omitempty"`


    StartDate Date `json:"start_date"`
}


type Date struct {
    time.Time
}


func NewDate(t time.Time) Date {
    return Date{Time: normalizeDate(t)}
}


func (d Date) MarshalJSON() ([]byte, error) {
    if d.Time.IsZero() {
        return []byte(`""`), nil
    }
    return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}


func (d *Date) UnmarshalJSON(data []byte) error {
    raw := string(data)
    if raw == `""` || raw == `null` {
        d.Time = time.Time{}
        return nil
    }


    t, err := time.Parse(`"2006-01-02"`, raw)
    if err != nil {
        return fmt.Errorf("invalid date format: expected YYYY-MM-DD")
    }


    d.Time = normalizeDate(t)
    return nil
}


func (s Status) Valid() bool {
    switch s {
    case StatusNew, StatusInProgress, StatusDone:
        return true
    default:
        return false
    }
}


func (t Task) Valid() bool {
    if t.Title == "" {
        return false
    }


    if !t.Status.Valid() {
        return false
    }


    if t.Recurrence != nil && !t.Recurrence.Valid() {
        return false
    }


    return true
}


func (r Recurrence) Valid() bool {
    if r.StartDate.Time.IsZero() {
        return false
    }


    switch r.Type {
    case RecurrenceTypeDaily:
        return r.validDaily()
    case RecurrenceTypeMonthlyDays:
        return r.validMonthlyDays()
    case RecurrenceTypeDates:
        return r.validDates()
    case RecurrenceTypeParity:
        return r.validParity()
    default:
        return false
    }
}


func (r Recurrence) OccursOn(date time.Time) bool {
    if !r.Valid() {
        return false
    }


    target := normalizeDate(date)
    start := normalizeDate(r.StartDate.Time)


    if target.Before(start) {
        return false
    }


    switch r.Type {
    case RecurrenceTypeDaily:
        diffDays := int(target.Sub(start).Hours() / 24)
        return diffDays%r.EveryNDays == 0


    case RecurrenceTypeMonthlyDays:
        day := target.Day()
        for _, d := range r.MonthDays {
            if d == day {
                return true
            }
        }
        return false


    case RecurrenceTypeDates:
        for _, d := range r.Dates {
            if sameDate(d.Time, target) {
                return true
            }
        }
        return false


    case RecurrenceTypeParity:
        day := target.Day()
        if r.Parity == DayParityEven {
            return day%2 == 0
        }
        return day%2 != 0


    default:
        return false
    }
}


func (r *Recurrence) Normalize() {
    if r == nil {
        return
    }


    r.StartDate = NewDate(r.StartDate.Time)


    switch r.Type {
    case RecurrenceTypeMonthlyDays:
        r.MonthDays = uniqueSortedDays(r.MonthDays)
    case RecurrenceTypeDates:
        r.Dates = uniqueSortedDates(r.Dates)
    }
}


func (r Recurrence) validDaily() bool {
    return r.EveryNDays > 0
}


func (r Recurrence) validMonthlyDays() bool {
    if len(r.MonthDays) == 0 {
        return false
    }


    for _, day := range r.MonthDays {
        if day < 1 || day > 30 {
            return false
        }
    }


    return true
}


func (r Recurrence) validDates() bool {
    if len(r.Dates) == 0 {
        return false
    }


    for _, d := range r.Dates {
        if d.Time.IsZero() {
            return false
        }
    }


    return true
}


func (r Recurrence) validParity() bool {
    return r.Parity == DayParityOdd || r.Parity == DayParityEven
}


func normalizeDate(t time.Time) time.Time {
    if t.IsZero() {
        return time.Time{}
    }
    return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}


func sameDate(a, b time.Time) bool {
    a = normalizeDate(a)
    b = normalizeDate(b)
    return a.Equal(b)
}


func uniqueSortedDays(days []int) []int {
    seen := make(map[int]struct{}, len(days))
    result := make([]int, 0, len(days))


    for _, day := range days {
        if _, ok := seen[day]; ok {
            continue
        }
        seen[day] = struct{}{}
        result = append(result, day)
    }


    sort.Ints(result)
    return result
}


func uniqueSortedDates(dates []Date) []Date {
    seen := make(map[string]struct{}, len(dates))
    result := make([]Date, 0, len(dates))


    for _, d := range dates {
        n := normalizeDate(d.Time)
        key := n.Format("2006-01-02")


        if _, ok := seen[key]; ok {
            continue
        }
        seen[key] = struct{}{}
        result = append(result, NewDate(n))
    }


    sort.Slice(result, func(i, j int) bool {
        return result[i].Time.Before(result[j].Time)
    })


    return result
} Вот целый task.go, сделай мне так, чтобы закрыть проблему валидации

Ниже — готовый task.go с детализированной валидацией и сохранением всей твоей логики.
Я добавил Validate() для Task и Recurrence, переписал Valid() поверх них и уточнил сообщения об ошибках.

go
package task

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type RecurrenceType string

const (
	RecurrenceTypeDaily       RecurrenceType = "daily"
	RecurrenceTypeMonthlyDays RecurrenceType = "monthly_days"
	RecurrenceTypeDates       RecurrenceType = "dates"
	RecurrenceTypeParity      RecurrenceType = "parity"
)

type DayParity string

const (
	DayParityOdd  DayParity = "odd"
	DayParityEven DayParity = "even"
)

type Task struct {
	ID          int64       `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      Status      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Recurrence  *Recurrence `json:"recurrence,omitempty"`
}

type Recurrence struct {
	Type RecurrenceType `json:"type"`

	EveryNDays int   `json:"every_n_days,omitempty"`
	MonthDays  []int `json:"month_days,omitempty"`
	Dates      []Date `json:"dates,omitempty"`
	Parity     DayParity `json:"parity,omitempty"`

	StartDate Date `json:"start_date"`
}

type Date struct {
	time.Time
}

func NewDate(t time.Time) Date {
	return Date{Time: normalizeDate(t)}
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + d.Time.Format("2006-01-02") + `"`), nil
}

func (d *Date) UnmarshalJSON(data []byte) error {
	raw := string(data)
	if raw == `""` || raw == `null` {
		d.Time = time.Time{}
		return nil
	}

	t, err := time.Parse(`"2006-01-02"`, raw)
	if err != nil {
		return fmt.Errorf("invalid date format for recurrence date: expected YYYY-MM-DD")
	}

	d.Time = normalizeDate(t)
	return nil
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

// Validate выполняет полную проверку задачи и возвращает конкретную ошибку.
func (t Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if !t.Status.Valid() {
		return fmt.Errorf("status is invalid")
	}

	if t.Recurrence != nil {
		if err := t.Recurrence.Validate(); err != nil {
			// без wrap ErrInvalidInput здесь, это делает usecase
			return err
		}
	}

	return nil
}

// Valid сохраняем для обратной совместимости, но теперь он использует Validate.
func (t Task) Valid() bool {
	return t.Validate() == nil
}

// Validate - детальная валидация настроек периодичности.
func (r Recurrence) Validate() error {
	if r.StartDate.Time.IsZero() {
		return fmt.Errorf("recurrence.start_date is required")
	}

	switch r.Type {
	case RecurrenceTypeDaily:
		if !r.validDaily() {
			return fmt.Errorf("recurrence.every_n_days must be greater than 0 for type 'daily'")
		}
		return nil

	case RecurrenceTypeMonthlyDays:
		if !r.validMonthlyDays() {
			return fmt.Errorf("recurrence.month_days must contain values between 1 and 30 for type 'monthly_days'")
		}
		return nil

	case RecurrenceTypeDates:
		if !r.validDates() {
			return fmt.Errorf("recurrence.dates must contain at least one valid date")
		}
		return nil

	case RecurrenceTypeParity:
		if !r.validParity() {
			return fmt.Errorf("recurrence.parity must be 'odd' or 'even' for type 'parity'")
		}
		return nil

	default:
		return fmt.Errorf("recurrence.type is invalid")
	}
}

// Valid сохраняем для обратной совместимости, но теперь он использует Validate.
func (r Recurrence) Valid() bool {
	return r.Validate() == nil
}

func (r Recurrence) OccursOn(date time.Time) bool {
	if !r.Valid() {
		return false
	}

	target := normalizeDate(date)
	start := normalizeDate(r.StartDate.Time)

	if target.Before(start) {
		return false
	}

	switch r.Type {
	case RecurrenceTypeDaily:
		diffDays := int(target.Sub(start).Hours() / 24)
		return diffDays%r.EveryNDays == 0

	case RecurrenceTypeMonthlyDays:
		day := target.Day()
		for _, d := range r.MonthDays {
			if d == day {
				return true
			}
		}
		return false

	case RecurrenceTypeDates:
		for _, d := range r.Dates {
			if sameDate(d.Time, target) {
				return true
			}
		}
		return false

	case RecurrenceTypeParity:
		day := target.Day()
		if r.Parity == DayParityEven {
			return day%2 == 0
		}
		return day%2 != 0

	default:
		return false
	}
}

func (r *Recurrence) Normalize() {
	if r == nil {
		return
	}

	r.StartDate = NewDate(r.StartDate.Time)

	switch r.Type {
	case RecurrenceTypeMonthlyDays:
		r.MonthDays = uniqueSortedDays(r.MonthDays)
	case RecurrenceTypeDates:
		r.Dates = uniqueSortedDates(r.Dates)
	}
}

func (r Recurrence) validDaily() bool {
	return r.EveryNDays > 0
}

func (r Recurrence) validMonthlyDays() bool {
	if len(r.MonthDays) == 0 {
		return false
	}

	for _, day := range r.MonthDays {
		if day < 1 || day > 30 {
			return false
		}
	}

	return true
}

func (r Recurrence) validDates() bool {
	if len(r.Dates) == 0 {
		return false
	}

	for _, d := range r.Dates {
		if d.Time.IsZero() {
			return false
		}
	}

	return true
}

func (r Recurrence) validParity() bool {
	return r.Parity == DayParityOdd || r.Parity == DayParityEven
}

func normalizeDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func sameDate(a, b time.Time) bool {
	a = normalizeDate(a)
	b = normalizeDate(b)
	return a.Equal(b)
}

func uniqueSortedDays(days []int) []int {
	seen := make(map[int]struct{}, len(days))
	result := make([]int, 0, len(days))

	for _, day := range days {
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		result = append(result, day)
	}

	sort.Ints(result)
	return result
}

func uniqueSortedDates(dates []Date) []Date {
	seen := make(map[string]struct{}, len(dates))
	result := make([]Date, 0, len(dates))

	for _, d := range dates {
		n := normalizeDate(d.Time)
		key := n.Format("2006-01-02")

		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, NewDate(n))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.Before(result[j].Time)
	})

	return result
}
Чтобы сообщения действительно стали понятными на уровне API, в usecase/сервисе вместо проверки Valid() нужно использовать Validate() и прокидывать текст наружу, например:

go
if model.Recurrence != nil {
    model.Recurrence.Normalize()
}

if err := model.Validate(); err != nil {
    return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
}
Тогда вместо "invalid task input: invalid recurrence" клиент увидит конкретику вроде "invalid input: recurrence.every_n_days must be greater than 0 for type 'daily'", и твой комментарий про “без конкретики” будет закрыт.

package task


import (
    "context"
    "fmt"
    "strings"
    "time"


    taskdomain "example.com/taskservice/internal/domain/task"
)


type Service struct {
    repo Repository
    now  func() time.Time
}


func NewService(repo Repository) *Service {
    return &Service{
        repo: repo,
        now:  func() time.Time { return time.Now().UTC() },
    }
}


func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
    normalized, err := validateCreateInput(input)
    if err != nil {
        return nil, err
    }


    model := &taskdomain.Task{
        Title:       normalized.Title,
        Description: normalized.Description,
        Status:      normalized.Status,
        Recurrence:  normalized.Recurrence,
    }


    now := s.now()
    model.CreatedAt = now
    model.UpdatedAt = now


    if model.Recurrence != nil {
        model.Recurrence.Normalize()
    }


    if !model.Valid() {
        return nil, fmt.Errorf("%w: invalid task", ErrInvalidInput)
    }


    created, err := s.repo.Create(ctx, model)
    if err != nil {
        return nil, err
    }


    return created, nil
}


func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
    if id <= 0 {
        return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
    }


    return s.repo.GetByID(ctx, id)
}


func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
    if id <= 0 {
        return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
    }


    normalized, err := validateUpdateInput(input)
    if err != nil {
        return nil, err
    }


    model := &taskdomain.Task{
        ID:          id,
        Title:       normalized.Title,
        Description: normalized.Description,
        Status:      normalized.Status,
        UpdatedAt:   s.now(),
        Recurrence:  normalized.Recurrence,
    }


    if model.Recurrence != nil {
        model.Recurrence.Normalize()
    }


    if !model.Valid() {
        return nil, fmt.Errorf("%w: invalid task", ErrInvalidInput)
    }


    updated, err := s.repo.Update(ctx, model)
    if err != nil {
        return nil, err
    }


    return updated, nil
}


func (s *Service) Delete(ctx context.Context, id int64) error {
    if id <= 0 {
        return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
    }


    return s.repo.Delete(ctx, id)
}


func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
    return s.repo.List(ctx)
}


func validateCreateInput(input CreateInput) (CreateInput, error) {
    input.Title = strings.TrimSpace(input.Title)
    input.Description = strings.TrimSpace(input.Description)


    if input.Title == "" {
        return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
    }


    if input.Status == "" {
        input.Status = taskdomain.StatusNew
    }


    if !input.Status.Valid() {
        return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
    }


    if input.Recurrence != nil {
        input.Recurrence.Normalize()
        if !input.Recurrence.Valid() {
            return CreateInput{}, fmt.Errorf("%w: invalid recurrence", ErrInvalidInput)
        }
    }


    return input, nil
}


func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
    input.Title = strings.TrimSpace(input.Title)
    input.Description = strings.TrimSpace(input.Description)


    if input.Title == "" {
        return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
    }


    if !input.Status.Valid() {
        return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
    }


    if input.Recurrence != nil {
        input.Recurrence.Normalize()
        if !input.Recurrence.Valid() {
            return UpdateInput{}, fmt.Errorf("%w: invalid recurrence", ErrInvalidInput)
        }
    }


    return input, nil
} вот service

Ниже — твой service.go, переписанный так, чтобы использовать новые Validate() из домена и возвращать конкретные сообщения об ошибках вместо общего invalid recurrence.

Ключевые изменения:

В Create/Update вместо model.Valid() вызывается model.Validate(), а текст ошибки прокидывается наружу.

В validateCreateInput/validateUpdateInput больше нет ручной проверки Recurrence.Valid() — это делает домен.

Сообщения валидации по-прежнему маркируются через ErrInvalidInput, чтобы маппинг на 400 остался прежним.

go
package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		Recurrence:  normalized.Recurrence,
	}

	now := s.now()
	model.CreatedAt = now
	model.UpdatedAt = now

	if model.Recurrence != nil {
		model.Recurrence.Normalize()
	}

	if err := model.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		ID:          id,
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		UpdatedAt:   s.now(),
		Recurrence:  normalized.Recurrence,
	}

	if model.Recurrence != nil {
		model.Recurrence.Normalize()
	}

	if err := model.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	updated, err := s.repo.Update(ctx, model)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.List(ctx)
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	// Recurrence здесь не валидируем подробно — это делает домен через Task.Validate().
	if input.Recurrence != nil {
		input.Recurrence.Normalize()
	}

	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.Recurrence != nil {
		input.Recurrence.Normalize()
	}

	return input, nil
}
Теперь при невалидной периодичности вместо "invalid task input: invalid recurrence" ты будешь получать, например:

invalid input: recurrence.start_date is required

invalid input: recurrence.every_n_days must be greater than 0 for type 'daily'

invalid input: recurrence.month_days must contain values between 1 and 30 for type 'monthly_days'

— то есть чёткую привязку к полю и типу периодичности, как от тебя просили.

