package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"tasktracker/internal/domain"
	"tasktracker/internal/service"
)

type CLI struct {
	tracker *service.Service
	in      *bufio.Scanner
	out     io.Writer
}

func New(tracker *service.Service, input io.Reader, output io.Writer) *CLI {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	return &CLI{
		tracker: tracker,
		in:      scanner,
		out:     output,
	}
}

func (c *CLI) Run(ctx context.Context) error {
	fmt.Fprintln(c.out, "Консольный таск-трекер. Введите help для списка команд.")

	for {
		fmt.Fprint(c.out, "> ")
		if !c.in.Scan() {
			if err := c.in.Err(); err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			return nil
		}

		line := strings.TrimSpace(c.in.Text())
		if line == "" {
			continue
		}

		args, err := tokenize(line)
		if err != nil {
			c.printError(err)
			continue
		}
		if len(args) == 0 {
			continue
		}

		exit, err := c.execute(ctx, args)
		if err != nil {
			c.printError(err)
		}
		if exit {
			return nil
		}
	}
}

func (c *CLI) execute(ctx context.Context, args []string) (bool, error) {
	switch strings.ToLower(args[0]) {
	case "help":
		c.printHelp()
		return false, nil
	case "exit", "quit":
		fmt.Fprintln(c.out, "До свидания!")
		return true, nil
	case "add":
		return false, c.add(ctx, args[1:])
	case "list":
		return false, c.list(ctx, args[1:])
	case "get":
		return false, c.get(ctx, args[1:])
	case "status":
		return false, c.changeStatus(ctx, args[1:])
	case "edit":
		return false, c.edit(ctx, args[1:])
	case "delete":
		return false, c.delete(ctx, args[1:])
	case "import":
		return false, c.importJSON(ctx, args[1:])
	default:
		return false, fmt.Errorf("unknown command %q; use help", args[0])
	}
}

func (c *CLI) add(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: add <title> <YYYY-MM-DD>")
	}
	deadlineText := args[len(args)-1]
	deadline, err := domain.ParseDeadline(deadlineText, time.Local)
	if err != nil {
		return fmt.Errorf("add: invalid deadline %q: %w", deadlineText, err)
	}
	title := strings.Join(args[:len(args)-1], " ")

	task, err := c.tracker.Create(ctx, title, deadline)
	if err != nil {
		return err
	}
	c.printTask(task)
	return nil
}

func (c *CLI) list(ctx context.Context, args []string) error {
	filter, err := parseListOptions(args)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	result, err := c.tracker.List(ctx, filter)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.out, "Найдено: %d | Страница %d/%d | limit=%d\n", result.Total, result.Page, result.TotalPages, result.Limit)
	if result.Total == 0 {
		fmt.Fprintln(c.out, "Задач нет.")
		return nil
	}
	for _, task := range result.Tasks {
		c.printTask(task)
	}
	return nil
}

func (c *CLI) get(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: get <id>")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	task, err := c.tracker.Get(ctx, id)
	if err != nil {
		return err
	}
	c.printTask(task)
	return nil
}

func (c *CLI) changeStatus(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: status <id> <planned|in_progress|canceled|done>")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	status, err := domain.ParseStatus(args[1])
	if err != nil {
		return err
	}
	task, err := c.tracker.ChangeStatus(ctx, id, status)
	if err != nil {
		return err
	}
	c.printTask(task)
	return nil
}

func (c *CLI) edit(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: edit <id> --title <title> [--deadline <YYYY-MM-DD>] or edit <id> --deadline <YYYY-MM-DD> [--title <title>]")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}

	var title *string
	var deadline *time.Time
	for i := 1; i < len(args); {
		switch args[i] {
		case "--title":
			if i+1 >= len(args) {
				return errors.New("edit: --title requires a value")
			}
			value := args[i+1]
			title = &value
			i += 2
		case "--deadline":
			if i+1 >= len(args) {
				return errors.New("edit: --deadline requires a value")
			}
			value, err := domain.ParseDeadline(args[i+1], time.Local)
			if err != nil {
				return fmt.Errorf("edit: invalid deadline: %w", err)
			}
			deadline = &value
			i += 2
		default:
			return fmt.Errorf("edit: unknown option %q", args[i])
		}
	}

	task, err := c.tracker.Edit(ctx, id, title, deadline)
	if err != nil {
		return err
	}
	c.printTask(task)
	return nil
}

func (c *CLI) delete(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: delete <id>")
	}
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	if err := c.tracker.Delete(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Задача %d удалена.\n", id)
	return nil
}

type importFile struct {
	Tasks []json.RawMessage `json:"tasks"`
}

type importFileTask struct {
	Title    string `json:"title"`
	Status   string `json:"status"`
	Deadline string `json:"deadline"`
}

type importItem struct {
	index int
	task  importFileTask
}

func (c *CLI) importJSON(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: import <path-to-json>")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("import: read file: %w", err)
	}

	items, issues, err := decodeImport(data)
	if err != nil {
		return fmt.Errorf("import: decode JSON: %w", err)
	}

	inputs := make([]service.TaskInput, 0, len(items))
	for _, item := range items {
		deadline, err := domain.ParseDeadline(item.task.Deadline, time.Local)
		if err != nil {
			issues = append(issues, service.ImportIssue{
				Index: item.index,
				Err:   fmt.Errorf("invalid deadline %q: %w", item.task.Deadline, err),
			})
			continue
		}

		inputs = append(inputs, service.TaskInput{
			Index:    item.index,
			Title:    item.task.Title,
			Status:   domain.Status(item.task.Status),
			Deadline: deadline,
		})
	}

	result := c.tracker.Import(ctx, inputs)
	result.Issues = append(issues, result.Issues...)
	sort.Slice(result.Issues, func(i, j int) bool {
		return result.Issues[i].Index < result.Issues[j].Index
	})

	fmt.Fprintf(c.out, "Импорт завершен: добавлено %d, ошибок %d.\n", len(result.Added), len(result.Issues))
	for _, issue := range result.Issues {
		fmt.Fprintf(c.out, "  Элемент %d: %s\n", issue.Index, issue.Err)
	}
	return nil
}

func decodeImport(data []byte) ([]importItem, []service.ImportIssue, error) {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		var wrapped importFile
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return nil, nil, err
		}
		rawItems = wrapped.Tasks
	}

	items := make([]importItem, 0, len(rawItems))
	issues := make([]service.ImportIssue, 0)
	for i, raw := range rawItems {
		var task importFileTask
		if err := json.Unmarshal(raw, &task); err != nil {
			issues = append(issues, service.ImportIssue{
				Index: i + 1,
				Err:   fmt.Errorf("invalid JSON object: %w", err),
			})
			continue
		}

		items = append(items, importItem{
			index: i + 1,
			task:  task,
		})
	}

	return items, issues, nil
}

func parseID(value string) (domain.TaskID, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id %q", value)
	}
	return domain.TaskID(id), nil
}

func parseListOptions(args []string) (service.ListFilter, error) {
	filter := service.ListFilter{Page: 1, Limit: 20}
	for i := 0; i < len(args); {
		switch args[i] {
		case "--status":
			if i+1 >= len(args) {
				return service.ListFilter{}, errors.New("--status requires a value")
			}
			status, err := domain.ParseStatus(args[i+1])
			if err != nil {
				return service.ListFilter{}, err
			}
			filter.Status = &status
			i += 2
		case "--overdue":
			filter.Overdue = true
			i++
		case "--search":
			if i+1 >= len(args) {
				return service.ListFilter{}, errors.New("--search requires a value")
			}
			filter.Search = args[i+1]
			i += 2
		case "--page":
			if i+1 >= len(args) {
				return service.ListFilter{}, errors.New("--page requires a value")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return service.ListFilter{}, errors.New("--page must be a positive integer")
			}
			filter.Page = value
			i += 2
		case "--limit":
			if i+1 >= len(args) {
				return service.ListFilter{}, errors.New("--limit requires a positive integer")
			}
			value, err := strconv.Atoi(args[i+1])
			if err != nil || value <= 0 {
				return service.ListFilter{}, errors.New("--limit must be a positive integer")
			}
			filter.Limit = value
			i += 2
		default:
			return service.ListFilter{}, fmt.Errorf("unknown option %q", args[i])
		}
	}
	return filter, nil
}

func (c *CLI) printHelp() {
	fmt.Fprintln(c.out, "Команды:")
	fmt.Fprintln(c.out, "  help")
	fmt.Fprintln(c.out, "      показать список команд")
	fmt.Fprintln(c.out, "  add <title> <YYYY-MM-DD>")
	fmt.Fprintln(c.out, "      добавить задачу; title можно взять в кавычки")
	fmt.Fprintln(c.out, "  list [--status <status>] [--overdue] [--search <text>] [--page N] [--limit N]")
	fmt.Fprintln(c.out, "      список задач, сортировка по дедлайну и ID")
	fmt.Fprintln(c.out, "  get <id>")
	fmt.Fprintln(c.out, "      получить одну задачу")
	fmt.Fprintln(c.out, "  status <id> <status>")
	fmt.Fprintln(c.out, "      изменить статус")
	fmt.Fprintln(c.out, "  edit <id> --title <title> --deadline <YYYY-MM-DD>")
	fmt.Fprintln(c.out, "      изменить title и/или deadline")
	fmt.Fprintln(c.out, "  delete <id>")
	fmt.Fprintln(c.out, "      удалить задачу")
	fmt.Fprintln(c.out, "  import <path-to-json>")
	fmt.Fprintln(c.out, "      импортировать задачи из JSON; некорректные элементы пропускаются")
	fmt.Fprintln(c.out, "  exit")
	fmt.Fprintln(c.out, "      завершить программу")
	fmt.Fprintln(c.out, "Статусы: planned, in_progress, canceled, done")
}

func (c *CLI) printTask(task domain.Task) {
	fmt.Fprintf(c.out, "[%d] %s | %s | %s\n", task.ID, task.Title, task.Status, domain.FormatDeadline(task.Deadline))
}

func (c *CLI) printError(err error) {
	fmt.Fprintf(c.out, "Ошибка: %s\n", err)
}
