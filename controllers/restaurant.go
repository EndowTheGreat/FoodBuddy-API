package controllers

import (
	"fmt"
	"foodbuddy/database"
	"foodbuddy/helper"
	"foodbuddy/model"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gocarina/gocsv"
	passwordvalidator "github.com/wagslane/go-password-validator"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// RestaurantSignup
func RestaurantSignup(c *gin.Context) {
	// bind json to struct
	var restaurantSignup model.RestaurantSignupRequest
	if err := c.BindJSON(&restaurantSignup); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "failed to process the request",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// validate input
	if err := helper.Validate(&restaurantSignup); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    err.Error(),
			"error_code": http.StatusBadRequest,
		})
		return
	}

	//check if the password and the confirm password is correct
	if restaurantSignup.Password != restaurantSignup.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  false,
			"message": "passwords doesn't match",
		})
		return
	}

	err := passwordvalidator.Validate(restaurantSignup.Password, model.PasswordEntropy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  false,
			"message": err.Error(),
		})
		return
	}

	// check if email exists
	var verification model.VerificationTable
	tx := database.DB.Where("email = ? AND role = ?", restaurantSignup.Email, model.RestaurantRole).First(&verification)
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "database error",
			"error_code": http.StatusInternalServerError,
		})
		return
	} else if tx.Error == gorm.ErrRecordNotFound {
		// create new entry in verification table
		verification = model.VerificationTable{
			Email:              restaurantSignup.Email,
			Role:               model.RestaurantRole,
			VerificationStatus: model.VerificationStatusPending,
		}
		tx = database.DB.Create(&verification)
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":     false,
				"message":    "failed to create restaurant verification entry",
				"error_code": http.StatusInternalServerError,
			})
			return
		}
	} else {
		// email already exists
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "restaurant email already exists",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// generate salt and hash password
	salt := helper.GenerateRandomString(7)
	saltedPassword := salt + restaurantSignup.Password
	hash, err := bcrypt.GenerateFromPassword([]byte(saltedPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to process the request",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	// create restaurant record
	restaurant := model.Restaurant{
		Name:               restaurantSignup.Name,
		Description:        restaurantSignup.Description,
		Address:            restaurantSignup.Address,
		Email:              restaurantSignup.Email,
		PhoneNumber:        restaurantSignup.PhoneNumber,
		ImageURL:           restaurantSignup.ImageURL,
		CertificateURL:     restaurantSignup.CertificateURL,
		VerificationStatus: model.VerificationStatusPending,
		Blocked:            false,
		Salt:               salt,
		HashedPassword:     string(hash),
	}

	// save to database
	if err := database.DB.Create(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to save restaurant data",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	// respond with success
	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "restaurant signup successful",
		"data": gin.H{
			"name":                restaurant.Name,
			"description":         restaurant.Description,
			"address":             restaurant.Address,
			"email":               restaurant.Email,
			"phone_number":        restaurant.PhoneNumber,
			"image_url":           restaurant.ImageURL,
			"certificate_url":     restaurant.CertificateURL,
			"verification_status": restaurant.VerificationStatus,
			"blocked":             restaurant.Blocked,
		},
	})
}

// RestaurantLogin
func RestaurantLogin(c *gin.Context) {
	// Get struct
	var restaurantLogin model.RestaurantLoginRequest
	var existingRestaurant model.Restaurant

	if err := c.BindJSON(&restaurantLogin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "Failed to process the incoming request",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// Validate
	if err := helper.Validate(&restaurantLogin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    err.Error(),
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// Check email on restaurant DB
	if err := database.DB.Where("email = ?", restaurantLogin.Email).First(&existingRestaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "Error fetching restaurant details",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	// Check block and admin verification status
	if existingRestaurant.Blocked {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":     false,
			"message":    "Restaurant not authorized to access the route",
			"error_code": http.StatusUnauthorized,
		})
		return
	}

	// Check password by salt and password
	password := []byte(existingRestaurant.Salt + restaurantLogin.Password)
	if err := bcrypt.CompareHashAndPassword([]byte(existingRestaurant.HashedPassword), password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":     false,
			"message":    "Invalid credentials",
			"error_code": http.StatusUnauthorized,
		})
		return
	}

	// Check email verification status using verification table
	var verificationTable model.VerificationTable
	if err := database.DB.Where("email = ?", restaurantLogin.Email).First(&verificationTable).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "Failed to fetch email verification status",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	if verificationTable.VerificationStatus != model.VerificationStatusVerified {
		if err := SendOTP(c, restaurantLogin.Email, verificationTable.OTPExpiry, model.RestaurantRole); err != nil {
			c.JSON(http.StatusAlreadyReported, gin.H{
				"status":     false,
				"message":    err.Error(),
				"error_code": http.StatusAlreadyReported,
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":     false,
			"message":    "Please verify your email to continue",
			"error_code": http.StatusUnauthorized,
		})
		return
	}

	token, err := GenerateJWT(c, existingRestaurant.Email, model.RestaurantRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "Failed to generate token",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	// Success
	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "Login is successful",
		"data": gin.H{
			"Name":               existingRestaurant.Name,
			"Email":              existingRestaurant.Email,
			"Description":        existingRestaurant.Description,
			"Address":            existingRestaurant.Address,
			"PhoneNumber":        existingRestaurant.PhoneNumber,
			"ImageURL":           existingRestaurant.ImageURL,
			"CertificateURL":     existingRestaurant.CertificateURL,
			"VerificationStatus": existingRestaurant.VerificationStatus,
		},
		"token": token,
	})
}

// public
func GetRestaurants(c *gin.Context) {
	var restaurants []model.Restaurant
	// Search db and get all
	if err := database.DB.Select("*").Find(&restaurants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to retrieve data from the database",
			"error_code": http.StatusInternalServerError,
		})
		return
	}
	var simplifiedRestaurants []gin.H

	for _, r := range restaurants {
		simplifiedRestaurants = append(simplifiedRestaurants, gin.H{
			"Name":               r.Name,
			"Email":              r.Email,
			"Description":        r.Description,
			"Address":            r.Address,
			"PhoneNumber":        r.PhoneNumber,
			"ImageURL":           r.ImageURL,
			"CertificateURL":     r.CertificateURL,
			"VerificationStatus": r.VerificationStatus,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "restaurants retrieved successfully",
		"data":    gin.H{"restaurantslist": simplifiedRestaurants},
	})
}

// restaurant
func EditRestaurant(c *gin.Context) {

	//check restaurant api authentication
	email, role, err := helper.GetJWTClaim(c)
	if role != model.RestaurantRole || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"message": "unauthorized request",
		})
		return
	}

	// Bind JSON
	var Request model.EditRestaurantRequest
	if err := c.BindJSON(&Request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "failed to bind request",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	if err := helper.Validate(Request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    err.Error(),
			"error_code": http.StatusBadRequest,
		})
		return
	}

	RestaurantID, ok := RestIDfromEmail(email)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"status":     false,
			"message":    "failed to retrieve restaurant information",
			"error_code": http.StatusNotFound,
		})
		return
	}

	// Check if present and update it with the new data
	var existingRestaurant model.Restaurant
	if err := database.DB.First(&existingRestaurant, RestaurantID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":     false,
			"message":    "restaurant doesn't exist",
			"error_code": http.StatusNotFound,
		})
		return
	}

	// Edit the restaurant
	if err := database.DB.Model(&existingRestaurant).Updates(Request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to edit the restaurant",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	fmt.Println("done")
	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "successfully edited the restaurant",
		"data":    Request,
	})
}

// admin
func DeleteRestaurant(c *gin.Context) {

	//check admin api authentication
	_, role, err := helper.GetJWTClaim(c)
	if role != model.AdminRole || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"message": "unauthorized request",
		})
		return
	}

	// Get the restaurant id
	restaurantIDStr := c.Param("restaurantid")
	restaurantID, err := strconv.Atoi(restaurantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "invalid restaurant ID",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// Check if it's already present
	var existingRestaurant model.Restaurant
	if err := database.DB.First(&existingRestaurant, restaurantID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":     false,
			"message":    "restaurant doesn't exist",
			"error_code": http.StatusNotFound,
		})
		return
	}

	// Delete it
	if err := database.DB.Delete(&existingRestaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to delete the restaurant",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "successfully deleted the restaurant",
		"data":    gin.H{},
	})
}

// admin
func BlockRestaurant(c *gin.Context) {
	//check admin api authentication
	_, role, err := helper.GetJWTClaim(c)
	if role != model.AdminRole || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"message": "unauthorized request",
		})
		return
	}

	// Get the restaurant id
	restaurantIDStr := c.Param("restaurantid")
	restaurantID, err := strconv.Atoi(restaurantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "invalid restaurant ID",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// Check restaurant by id
	var restaurant model.Restaurant
	if err := database.DB.First(&restaurant, restaurantID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":     false,
			"message":    "restaurant not found",
			"error_code": http.StatusNotFound,
		})
		return
	}

	if restaurant.Blocked {
		c.JSON(http.StatusAlreadyReported, gin.H{
			"status":     false,
			"message":    "restaurant is already blocked",
			"error_code": http.StatusConflict,
			"data": gin.H{
				"name":        restaurant.Name,
				"email":       restaurant.Email,
				"address":     restaurant.Address,
				"blockstatus": restaurant.Blocked,
			},
		})
		return
	}

	// Set blocked as true
	restaurant.Blocked = true

	if err := database.DB.Save(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to change the block status",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "restaurant is blocked",
		"data": gin.H{
			"name":        restaurant.Name,
			"email":       restaurant.Email,
			"address":     restaurant.Address,
			"blockstatus": restaurant.Blocked,
		},
	})
}

// admin
func UnblockRestaurant(c *gin.Context) {
	//check admin api authentication
	_, role, err := helper.GetJWTClaim(c)
	if role != model.AdminRole || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"message": "unauthorized request",
		})
		return
	}

	// Get the restaurant id
	restaurantIDStr := c.Param("restaurantid")
	restaurantID, err := strconv.Atoi(restaurantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     false,
			"message":    "invalid restaurant ID",
			"error_code": http.StatusBadRequest,
		})
		return
	}

	// Check restaurant by id
	var restaurant model.Restaurant
	if err := database.DB.First(&restaurant, restaurantID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":     false,
			"message":    "restaurant not found",
			"error_code": http.StatusNotFound,
		})
		return
	}

	if !restaurant.Blocked {
		c.JSON(http.StatusConflict, gin.H{
			"status":     false,
			"message":    "restaurant is already unblocked",
			"error_code": http.StatusConflict,
			"data": gin.H{
				"name":        restaurant.Name,
				"email":       restaurant.Email,
				"address":     restaurant.Address,
				"blockstatus": restaurant.Blocked,
			},
		})
		return
	}

	// Set blocked as false
	restaurant.Blocked = false

	if err := database.DB.Save(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":     false,
			"message":    "failed to change the block status",
			"error_code": http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "restaurant is unblocked",
		"data": gin.H{
			"name":        restaurant.Name,
			"email":       restaurant.Email,
			"address":     restaurant.Address,
			"blockstatus": restaurant.Blocked,
		},
	})
}

//get restaurantid from rest email

func RestIDfromEmail(email string) (uint, bool) {
	var Restaurant model.Restaurant
	if err := database.DB.Where("email = ?", email).First(&Restaurant).Error; err != nil {
		return 0, false
	}
	return Restaurant.ID, true
}

func RestaurantWalletBalance(email string) (float64, bool) {
	var Restaurant model.Restaurant
	if err := database.DB.Where("email = ?", email).First(&Restaurant).Error; err != nil {
		return 0, false
	}
	return Restaurant.WalletAmount, true
}

func GetRestaurantWalletData(c *gin.Context) {
	//check restaurant api authentication
	email, role, err := helper.GetJWTClaim(c)
	if role != model.RestaurantRole || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"message": "unauthorized request",
		})
		return
	}

	RestID, _ := RestIDfromEmail(email)
	WalletBalance, _ := RestaurantWalletBalance(email)

	var Result []model.RestaurantWalletHistory
	if err := database.DB.Where("restaurant_id = ?", RestID).Find(&Result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": false, "message": "failed to get restaurant wallet history"})
		return
	}

	type Response struct {
		WalletBalance float64
		History       []model.RestaurantWalletHistory
	}

	c.JSON(http.StatusOK, gin.H{
		"status": true,
		"data": Response{
			WalletBalance: WalletBalance,
			History:       Result,
		},
	})
}


func OrderInformationsCSVFileForRestaurant(c *gin.Context) {
	email, role, err := helper.GetJWTClaim(c)
	if role!= model.RestaurantRole || err!= nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  false,
			"message": "unauthorized request",
		})
		return
	}

	RestID, _ := RestIDfromEmail(email)

	var OrderInformation []model.OrderItem
	err = database.DB.Where("restaurant_id =?", RestID).Find(&OrderInformation).Error
	if err!= nil {
		log.Printf("Failed to retrieve order information: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve order information"})
		return
	}

	if len(OrderInformation) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": true, "message": "No orders found for this restaurant."})
		return
	}

	data := make([]model.OrderItem, len(OrderInformation))
	for i, item := range OrderInformation {
		data[i] = model.OrderItem{
			OrderID:            item.OrderID,
			UserID:             uint(item.UserID),
			RestaurantID:       uint(item.RestaurantID),
			ProductID:          uint(item.ProductID),
			Quantity:           item.Quantity,
			Amount:             item.Amount,
			ProductOfferAmount: item.ProductOfferAmount,
			AfterDeduction:     item.AfterDeduction,
			CookingRequest:     item.CookingRequest,
			OrderStatus:        item.OrderStatus,
			OrderReview:        item.OrderReview,
			OrderRating:        item.OrderRating,
		}
	}

	tmpfile, err := os.CreateTemp("", "orders_*.csv")
	if err!= nil {
		log.Printf("Failed to create temp file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create CSV file"})
		return
	}
	defer os.Remove(tmpfile.Name()) 

	if err := gocsv.MarshalFile(data, tmpfile); err!= nil {
		log.Printf("Failed to marshal CSV data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write CSV data"})
		return
	}
	c.Writer.Header().Set("Content-Type", "text/csv")
	c.Writer.Header().Set("Content-Disposition", "attachment; filename=orders.csv")
	c.File(tmpfile.Name())
}

func OrderPaymentInformationsExcelFile(c *gin.Context) {

}
