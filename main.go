package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type RecordExcel struct {
	Tipe  string
	Tahun string
	Harga int64
}
type CollateralType struct {
	ID                int64  `json:"id"`
	CollateralBrandID int64  `json:"collateral_brand_id"`
	Name              string `json:"Name"`
	Tahun             string `json:"tahun"`
	IsActive          bool   `json:"is_active"`
	CreatedBy         int64  `json:"created_by"`
}

type CollateralMotorcyclePrice struct {
	ID                int64 `json:"id"`
	CollateralBrandID int64 `json:"collateral_brand_id"`
	CollateralTypeID  int64 `json:"collateral_type_id"`
	MstProvinceID     int64 `json:"mst_province_id"`
	BasePrice         int64 `json:"base_price"`
	CreatedBy         int64 `json:"created_by"`
}

func readExcel(filepath string) ([]RecordExcel, error) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return nil, err
	}

	var records []RecordExcel
	for i, row := range rows {
		if i == 0 {
			continue // Skip header
		}

		harga, _ := strconv.Atoi(row[3]) // Convert harga to int
		records = append(records, RecordExcel{
			Tipe:  row[1],
			Tahun: row[2],
			Harga: int64(harga),
		})
	}

	return records, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get environment variables
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName, dbPort)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	records, err := readExcel("data.xlsx")
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Semaphore untuk membatasi goroutine, misalnya max 10 goroutine aktif sekaligus.

	for _, record := range records {
		wg.Add(1)
		sem <- struct{}{} // Mengisi slot semaphore

		go func(record RecordExcel) {
			defer wg.Done()
			defer func() { <-sem }() // Membebaskan slot semaphore

			var collateralType CollateralType

			// Cek apakah tipe dan tahun sudah ada
			err := db.Where("name = ? AND tahun = ?", record.Tipe, record.Tahun).
				First(&collateralType).Error

			if err != nil {
				// Jika tidak ada, buat collateral_type baru
				collateralType = CollateralType{
					CollateralBrandID: 1,
					IsActive:          true,
					CreatedBy:         1,
					Name:              record.Tipe,
					Tahun:             record.Tahun,
				}
				if err := db.Create(&collateralType).Error; err != nil {
					log.Printf("Failed to insert collateral type: %v", err)
					return
				}
			}

			// Simpan harga ke collateral_motorcycle_prices
			collateralPrice := CollateralMotorcyclePrice{
				CollateralBrandID: 1,
				MstProvinceID:     0,
				CollateralTypeID:  collateralType.ID,
				BasePrice:         record.Harga,
				CreatedBy:         1,
			}
			if err := db.Create(&collateralPrice).Error; err != nil {
				log.Printf("Failed to insert collateral price: %v", err)
			}
		}(record)
	}

	wg.Wait()
	log.Println("Data berhasil disimpan!")
}
