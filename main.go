package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

type Storage interface {
	Get(key string) (value string, err error)
	Set(key, value string) (err error)
}

// memory
type MemStorage struct {
	m map[string]string
}

func (ms *MemStorage) Get(key string) (value string, err error) {
	var ok bool

	value, ok = ms.m[key]
	if !ok {
		return value, fmt.Errorf("err not found")
	}
	return value, nil
}

func (ms *MemStorage) Set(key, value string) (err error) {
	ms.m[key] = value
	return nil
}
func NewMemStorage() Storage { // обрати внимание, что возвращаем интерфейс
	return &MemStorage{m: make(map[string]string)}
}

// file
type FileStorage struct {
	ms *MemStorage // сделаем внутреннюю хранилку в памяти тоже интерфейсом, на случай если захотим ее замокать
	f  *os.File
}

func (fs *FileStorage) Get(key string) (value string, err error) {
	return fs.ms.Get(key)
}

func (fs *FileStorage) Set(key, value string) (err error) {
	if err = fs.ms.Set(key, value); err != nil {
		return fmt.Errorf("unable to add new key in memorystorage: %w", err)
	}

	// перезаписываем файл с нуля
	err = fs.f.Truncate(0)
	if err != nil {
		return fmt.Errorf("unable to truncate file: %w", err)
	}
	_, err = fs.f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("unable to get the beginning of file: %w", err)
	}

	err = json.NewEncoder(fs.f).Encode(&fs.ms.m)
	if err != nil {
		return fmt.Errorf("unable to encode data into the file: %w", err)
	}
	return nil
}
func NewFileStorage(filename string) (Storage, error) { // и здесь мы тоже возвраащем интерфейс
	// мы открываем (или создаем файл если он не существует (os.O_CREATE)), в режиме чтения и записи (os.O_RDWR) и дописываем в конец (os.O_APPEND)
	// у созданного файла будут права 0777 - все пользователи в системе могут его читать, изменять и исполнять
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		return nil, fmt.Errorf("unable to open file %s: %w", filename, err)
	}

	// восстанавливаем данные из файла, мы будем их хранить в формате JSON
	m := make(map[string]string)
	if err := json.NewDecoder(file).Decode(&m); err != nil && err != io.EOF { // проверка на io.EOF тк файл может быть пустой
		return nil, fmt.Errorf("unable to decode contents of file %s: %w", filename, err)
	}

	return &FileStorage{
		ms: &MemStorage{m: m},
		f:  file,
	}, nil
}

// example handler
func getHandler(s Storage) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]

		value, err := s.Get(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.Write([]byte(value))
	}
}

// example handler
func postHandler(s Storage) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]
		value := vars["value"]

		if err := s.Set(key, value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(value))
	}
}

func main() {
	fileStorage, err := NewFileStorage("somefile.json")
	if err != nil {
		log.Fatalf("unable to create file storage: %v", err)
	}
	memStorage := NewMemStorage()

	r := mux.NewRouter()

	r.HandleFunc("/file/{key}", getHandler(fileStorage)).Methods(http.MethodGet)
	r.HandleFunc("/memory/{key}", getHandler(memStorage)).Methods(http.MethodGet)

	r.HandleFunc("/file/{key}/{value}", postHandler(fileStorage)).Methods(http.MethodPost)
	r.HandleFunc("/memory/{key}/{value}", postHandler(memStorage)).Methods(http.MethodPost)

	log.Fatal(http.ListenAndServe(":8080", r))
}

