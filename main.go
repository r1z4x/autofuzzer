package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

type FuzzTask struct {
	Template string
	Wordlist string
	Params   []string
}

func main() {
	url := "http://localhost:8080" // Taramak istediğiniz URL

	// Özelleştirilebilir şablon ve wordlist kombinasyonları
	tasks := []FuzzTask{
		{Template: "/FUZZ.txt", Wordlist: "./raft-small-directories.txt"},
		{Template: "/FUZZ.php", Wordlist: "./raft-small-directories.txt"},
		{Template: "/FUZZ.html", Wordlist: "./raft-small-directories.txt"},
		// Daha fazla şablon ve wordlist ekleyebilirsiniz
	}

	threads := 5 // Paralel olarak çalıştırılacak iş parçacığı sayısı

	results, err := fuzzTemplates(url, tasks, threads)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Optimize Edilmiş Sonuçlar:")
	for _, result := range results {
		fmt.Println(result)
	}
}

func fuzzTemplates(url string, tasks []FuzzTask, threads int) ([]string, error) {
	var results []string
	var resultsMutex sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, threads)
	for _, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}

		go func(task FuzzTask) {
			defer wg.Done()

			restartLimit := 3
			restartCount := 0

			for {
				shouldRestart := false
				if restartCount >= restartLimit {
					log.Printf("Tekrarlayan benzerlikler nedeniyle tarama %d kez yeniden başlatıldı. Lütfen görev ayarlarını kontrol edin.\n", restartLimit)
					break
				}

				cmdArgs := []string{"-w", task.Wordlist, "-u", url + task.Template, "-json"}
				cmdArgs = append(cmdArgs, task.Params...)
				cmd := exec.Command("ffuf", cmdArgs...)

				stdout, err := cmd.StdoutPipe()
				if err != nil {
					log.Printf("stdout hatası: %v\n", err)
					<-sem
					return
				}

				if err := cmd.Start(); err != nil {
					log.Printf("ffuf hatası: %v\n", err)
					<-sem
					return
				}

				type FfufResult struct {
					URL           string `json:"url"`
					StatusCode    int    `json:"status"`
					ContentLength int64  `json:"content_length"`
					ContentType   string `json:"content_type"`
				}

				resultsCount := make(map[string]int)
				duplicatesThreshold := 30
				scanner := bufio.NewScanner(stdout)
				startTime := time.Now()
				d := 0
				for scanner.Scan() {
					line := scanner.Text()
					if len(line) > 0 {
						var result FfufResult
						if err := json.Unmarshal([]byte(line), &result); err != nil {
							log.Printf("JSON hatası: %v\n", err)
							continue
						}

						resultsMutex.Lock()
						results = append(results, fmt.Sprintf("%s, %d, %d, %s", result.URL, result.StatusCode, result.ContentLength, result.ContentType))
						resultsMutex.Unlock()

						// Sonuçları analiz edin ve tekrarlayan benzerlikleri sayın
						responseContent := fmt.Sprintf("%d,%s", result.StatusCode, result.ContentType)

						resultsCount[responseContent]++

						// Eşleşen yanıt içeriği sayısını kontrol edin
						if resultsCount[responseContent] >= duplicatesThreshold {
							// Yeni filtreleme parametrelerini ekle
							task.Params = append(task.Params, "-fc", responseContent)

							// Sürekli tekrarlayan benzerlikleri otomatik olarak filtreleyip taramayı tekrar başlatın
							log.Println("Tekrarlayan benzerlikler tespit edildi. Tarama tekrar başlatılıyor...")
							results = []string{} // results'u sıfırla
							shouldRestart = true
							break
						}
					}
				}

				if err := cmd.Wait(); err != nil {
					log.Printf("ffuf hatası: %v\n", err)
					<-sem
					return
				}

				fmt.Println("Adet : %", d)

				elapsed := time.Since(startTime)

				if elapsed < 30*time.Second {
					// Tarama süresi 30 saniyeden kısa ise, sürekli tekrarlayan benzerlikler yoktur ve tarama tamamlanmıştır.
					break
				} else if shouldRestart {

					// Filtrelenmiş sonuçlarla taramayı tekrar başlatın
					restartCount++
					time.Sleep(1 * time.Second)
				} else {
					break
				}
			}

			<-sem
		}(task)
	}

	wg.Wait()
	return results, nil
}
