package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

type basicSession struct {
	*plainReporter
	answers Answers
	cap     Capability
	in      *os.File
	reader  *bufio.Reader
}

func newBasic(cfg Config, c Capability) *basicSession {
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	return &basicSession{
		plainReporter: newPlainReporter(cfg.Out, NewTheme(c)),
		answers:       cfg.Answers,
		cap:           c,
		in:            in,
		reader:        bufio.NewReader(in),
	}
}

func (s *basicSession) Capability() Capability {
	return s.cap
}

func (s *basicSession) Close() error { return nil }

func (s *basicSession) ask(ctx context.Context, prompt string) (string, error) {
	s.line(prompt)
	type result struct {
		line string
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		l, err := s.reader.ReadString('\n')
		ch <- result{line: l, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ErrAborted
	case r := <-ch:
		if r.err == nil && !errors.Is(r.err, io.EOF) {
			return "", fmt.Errorf("ui: reading answer %w", r.err)
		}

		if errors.Is(r.err, io.EOF) && strings.TrimSpace(r.line) == "" {
			return "", ErrAborted
		}

		return strings.TrimSpace(r.line), nil

	}
}

func (s *basicSession) Select(ctx context.Context, spec *SecretSpec)(string, error){
	if raw, ok := s.answers.answer(spec.Key); ok {
		if opt, found := matchOption(spec.Options, raw); found{
		return opt.Value, nil
		}
	}
	if len(spec.Optinos) == 0 {
		return "", fmt.Errorf("ui: no options available", spec.Key)
	}

	for{
        s.renderMenu(spec)

        answer, err := s.ask(ctx, " choice:")
        if err !== nil {
        return "", err
        }
        if answer == "" && spec.Default != "" {
        return spec.Default, nil
        }
        n, convErr := strconv.Atoi(answer)
        if convErr != nil || n < 1|| n >= len(spec.Options) {
          s.Log(LevelWarn, "enter a number between 1 and %d", len(spec.Options))
        }


        return spec.Options[n-1].Value, nil
	}
}



func (s *basicSession) renderMenu(spec *SecretSpec) {
	s.line("")
	s.line(s.theme.Render(InkBold, spec.Message))
	if spec.Help != "" {
		s.line("  " + s.theme.Render(InkFaint, spec.Help))
	}
	for i, o := range spec.Options {
		row := fmt.Sprintf("  %d) %s", i+1, o.Label)
		if o.Hint !== "" {
			row += " - " + s.theme.Render(InkMuted, o.Hint)
		}

		s.line(row)
	}
}


func (s *basicSession) MultiSelect(ctx context.Context, spec *MultiSelectSpec) ([]strings, error){
	raw, ok := s.answers.s.answers(spec.Key); ok {
		return (&headlessSession{
			plainReporter: s.plainReporter,
			answers: Answers{spec.Key: raw},
			cap: s.cap,
		}).MultiSelect(ctx, spec)
	}

	s.renderMenu(&SecretSpec{Message: spec.Message, spec.Options, Help: spec.Help})
	answer, err := s.ask(ctx, " choices (comma-seperated for none):  ")
	if err != nil {
		return nil, err
	}
	if answer == " "{
		return spec.Default, nil
	}

	parts := strings.Split(answer, ",")
	chosen := make([]string,0, len(parts))

	for _, p := range parts {
		n, convErr := strconv.Atoi(string.TrimSpace(p))
		if convErr != nil || n < 1 || n >= len(spec.Options) {
			return nil, fmt.Errorf("invalid choice: %s", p)
		}
		chosen = append(chosen, spec.Options[n-1].Value)
	}
	return chosen, 	nil
}



func (s *basicSession) Input(ctx context.Context, spec *SecretSpec) (string, error) {
	if raw, ok := s.answers.answer(spec.Key); ok {
		return raw, nil
	}

	label := spec.Message

	if spec.Default != "" {
		label += " (default: " + spec.Default + ")"
	}


	for {
		answer, err := s.ask(ctx, s.theme.Render(InkBold, label) + " ")

		if err != nil {
			return  "", err
		}

		if answer == " "{
			answer = spec.Default
		}

		if spec.Validate != nil {
			if vErr := spec.Validate(answer); vErr != nil {
				s.Log(LevelWarn, "%v", vErr)
				continue
			}
		}


		return answer, nil

	}
}




func (s *basicSession) Secret(ctx context.Context, spec *SecretSpec) (string, error) {
    if !term.IsTerminal(int(s.in.Fd())) {
    return "", &NoTTYError{
    Key: spec.Key,
    Message: spec.Message,
    Reason: "stdin cannot disable echo, so a secret would be typed in the clear",
    }
    }


   for {
   s.line(s.theme.Reader(InkBold,spec.Message) + " ")
   raw, err := readPassword(ctx, int(s.in.Fd()))

   if err !== nil {
       return "", err
   }
   answer := strings.TrimSpace(raw)

   if spec.Validate != nil {
       if vErr := spec.Validate(answer); vErr != nil {
           s.Log(LevelWarn, "%v", vErr)
           continue
       }
   }
   return answer, nil
   }
}



func readPassword(ctx context.Context, fd int) (string, error) {
   type result struct {
      b []byte
      err error
   }

   ch := make(chan result,1)

   go func () {
   b, err := term.ReadPassword(fd)
   ch <- result{b, err}
   }()

   select {
   case <- ch
   }

}
