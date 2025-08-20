// Package validator provides comprehensive data validation with extensive rules
// for strings, numbers, dates, financial data, and complex data structures.
// It offers both struct tag-based validation and programmatic validation
// with detailed error reporting and internationalization support.
//
// # Features
//
//   - Extensive validation rules for common data types
//   - Struct tag-based validation for easy integration
//   - Programmatic validation API for complex scenarios
//   - Financial validation (credit cards, currencies, etc.)
//   - Date and time validation with format support
//   - String validation (email, URL, phone, etc.)
//   - Numeric validation with range and precision checks
//   - Collection validation for arrays and slices
//   - UUID and identifier validation
//   - Password strength validation
//   - Custom validation rules support
//   - Internationalization and localization
//   - Detailed error messages with field context
//
// # Basic Usage
//
// Validate structs using tags:
//
//	import "github.com/dmitrymomot/gokit/core/validator"
//
//	type User struct {
//		Name  string `validate:"required,min=2,max=50"`
//		Email string `validate:"required,email"`
//		Age   int    `validate:"min=18,max=120"`
//	}
//
//	user := User{
//		Name:  "John Doe",
//		Email: "john@example.com",
//		Age:   25,
//	}
//
//	v := validator.New()
//	if err := v.Struct(user); err != nil {
//		// Handle validation errors
//		for _, fieldErr := range err.(validator.ValidationErrors) {
//			fmt.Printf("Field: %s, Error: %s\n", fieldErr.Field(), fieldErr.Tag())
//		}
//	}
//
// # String Validation
//
// Validate strings with various rules:
//
//	type Profile struct {
//		Username string `validate:"required,alphanum,min=3,max=20"`
//		Email    string `validate:"required,email"`
//		Website  string `validate:"omitempty,url"`
//		Phone    string `validate:"omitempty,phone"`
//		Bio      string `validate:"max=500"`
//	}
//
//	// Programmatic validation
//	v := validator.New()
//
//	// Email validation
//	if !v.IsEmail("user@example.com") {
//		// Handle invalid email
//	}
//
//	// URL validation
//	if !v.IsURL("https://example.com") {
//		// Handle invalid URL
//	}
//
//	// Phone validation
//	if !v.IsPhone("+1-555-123-4567") {
//		// Handle invalid phone
//	}
//
// # Numeric Validation
//
// Validate numeric values with range and precision checks:
//
//	type Product struct {
//		Price    float64 `validate:"required,min=0.01,max=10000"`
//		Quantity int     `validate:"required,min=1"`
//		Rating   float64 `validate:"min=1,max=5"`
//		Discount int     `validate:"min=0,max=100"` // Percentage
//	}
//
//	// Programmatic numeric validation
//	if !v.IsInRange(price, 0.01, 10000) {
//		// Handle price out of range
//	}
//
//	if !v.IsPositive(quantity) {
//		// Handle negative quantity
//	}
//
// # Date and Time Validation
//
// Validate dates and timestamps:
//
//	type Event struct {
//		StartDate string `validate:"required,datetime=2006-01-02"`
//		EndDate   string `validate:"required,datetime=2006-01-02,gtefield=StartDate"`
//		CreatedAt string `validate:"required,rfc3339"`
//	}
//
//	// Programmatic date validation
//	if !v.IsDate(dateString, "2006-01-02") {
//		// Handle invalid date format
//	}
//
//	if !v.IsAfter(endDate, startDate) {
//		// Handle invalid date range
//	}
//
// # Financial Validation
//
// Validate financial data:
//
//	type Payment struct {
//		CardNumber string `validate:"required,credit_card"`
//		CVV        string `validate:"required,len=3"`
//		Amount     string `validate:"required,currency"`
//		IBAN       string `validate:"omitempty,iban"`
//	}
//
//	// Programmatic financial validation
//	if !v.IsCreditCard("4111-1111-1111-1111") {
//		// Handle invalid credit card
//	}
//
//	if !v.IsIBAN("GB82WEST12345698765432") {
//		// Handle invalid IBAN
//	}
//
//	if !v.IsCurrency("USD") {
//		// Handle invalid currency code
//	}
//
// # Collection Validation
//
// Validate arrays, slices, and nested structures:
//
//	type Order struct {
//		ID    string      `validate:"required,uuid"`
//		Items []OrderItem `validate:"required,min=1,dive"`
//		Tags  []string    `validate:"max=10,dive,max=20"`
//	}
//
//	type OrderItem struct {
//		ProductID string  `validate:"required,uuid"`
//		Quantity  int     `validate:"required,min=1"`
//		Price     float64 `validate:"required,min=0"`
//	}
//
//	// Validate nested structures
//	order := Order{
//		ID: "123e4567-e89b-12d3-a456-426614174000",
//		Items: []OrderItem{
//			{
//				ProductID: "987fcdeb-51d2-43a8-b123-456789abcdef",
//				Quantity:  2,
//				Price:     19.99,
//			},
//		},
//		Tags: []string{"electronics", "gadgets"},
//	}
//
//	if err := v.Struct(order); err != nil {
//		// Handle validation errors
//	}
//
// # Password Validation
//
// Validate password strength:
//
//	type Registration struct {
//		Username string `validate:"required,min=3,max=20,alphanum"`
//		Password string `validate:"required,password_strong"`
//		Email    string `validate:"required,email"`
//	}
//
//	// Programmatic password validation
//	if !v.IsStrongPassword("MySecureP@ssw0rd!") {
//		// Handle weak password
//	}
//
//	// Custom password requirements
//	requirements := validator.PasswordRequirements{
//		MinLength:    8,
//		MaxLength:    100,
//		RequireUpper: true,
//		RequireLower: true,
//		RequireDigit: true,
//		RequireSymbol: true,
//	}
//
//	if !v.ValidatePassword("password", requirements) {
//		// Handle password that doesn't meet requirements
//	}
//
// # UUID and Identifier Validation
//
// Validate various identifier formats:
//
//	type Resource struct {
//		ID       string `validate:"required,uuid4"`
//		ParentID string `validate:"omitempty,uuid"`
//		Slug     string `validate:"required,slug"`
//		Handle   string `validate:"required,handle"`
//	}
//
//	// Programmatic identifier validation
//	if !v.IsUUID("123e4567-e89b-12d3-a456-426614174000") {
//		// Handle invalid UUID
//	}
//
//	if !v.IsSlug("my-blog-post-title") {
//		// Handle invalid slug
//	}
//
// # Custom Validation Rules
//
// Create custom validation rules:
//
//	// Register custom validator
//	v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
//		username := fl.Field().String()
//		// Custom username validation logic
//		return len(username) >= 3 && isValidUsernameChars(username)
//	})
//
//	// Use custom validator in struct
//	type User struct {
//		Username string `validate:"required,username"`
//	}
//
//	// Custom validation function
//	func validateBusinessRules(user User) error {
//		if user.IsAdmin && user.Department != "IT" {
//			return errors.New("admin users must be in IT department")
//		}
//		return nil
//	}
//
// # Conditional Validation
//
// Validate fields based on conditions:
//
//	type Address struct {
//		Country    string `validate:"required"`
//		State      string `validate:"required_if=Country US"`
//		PostalCode string `validate:"required_if=Country US,numeric"`
//		City       string `validate:"required"`
//	}
//
//	type User struct {
//		Type          string `validate:"required,oneof=individual business"`
//		CompanyName   string `validate:"required_if=Type business"`
//		TaxID         string `validate:"required_if=Type business"`
//		PersonalName  string `validate:"required_if=Type individual"`
//	}
//
// # Error Handling and Messages
//
// Handle validation errors with detailed messages:
//
//	func handleValidationErrors(err error) map[string]string {
//		errors := make(map[string]string)
//
//		for _, fieldErr := range err.(validator.ValidationErrors) {
//			field := fieldErr.Field()
//			tag := fieldErr.Tag()
//			param := fieldErr.Param()
//
//			switch tag {
//			case "required":
//				errors[field] = fmt.Sprintf("%s is required", field)
//			case "email":
//				errors[field] = "Must be a valid email address"
//			case "min":
//				errors[field] = fmt.Sprintf("Must be at least %s characters", param)
//			case "max":
//				errors[field] = fmt.Sprintf("Must be at most %s characters", param)
//			default:
//				errors[field] = fmt.Sprintf("Invalid %s", field)
//			}
//		}
//
//		return errors
//	}
//
// # Internationalization
//
// Support multiple languages for error messages:
//
//	// Register translations
//	uni := ut.New(en.New(), en.New(), es.New(), fr.New())
//	trans, _ := uni.GetTranslator("en")
//
//	// Register validator with translator
//	v := validator.New()
//	en_translations.RegisterDefaultTranslations(v, trans)
//
//	// Validate with translated errors
//	err := v.Struct(user)
//	if err != nil {
//		for _, fieldErr := range err.(validator.ValidationErrors) {
//			fmt.Println(fieldErr.Translate(trans))
//		}
//	}
//
// # API Validation Middleware
//
// Use validation in HTTP middleware:
//
//	func validationMiddleware(v *validator.Validate) func(http.HandlerFunc) http.HandlerFunc {
//		return func(next http.HandlerFunc) http.HandlerFunc {
//			return func(w http.ResponseWriter, r *http.Request) {
//				// Parse request body
//				var data RequestData
//				if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
//					http.Error(w, "Invalid JSON", http.StatusBadRequest)
//					return
//				}
//
//				// Validate data
//				if err := v.Struct(data); err != nil {
//					errors := handleValidationErrors(err)
//					response := map[string]interface{}{
//						"error": "Validation failed",
//						"details": errors,
//					}
//					w.Header().Set("Content-Type", "application/json")
//					w.WriteHeader(http.StatusBadRequest)
//					json.NewEncoder(w).Encode(response)
//					return
//				}
//
//				// Store validated data in context
//				ctx := context.WithValue(r.Context(), "validatedData", data)
//				next(w, r.WithContext(ctx))
//			}
//		}
//	}
//
// # Best Practices
//
//   - Use struct tags for simple validation rules
//   - Implement custom validators for business logic
//   - Provide clear, user-friendly error messages
//   - Validate data at application boundaries (API, forms, etc.)
//   - Use conditional validation for complex scenarios
//   - Sanitize input before validation when necessary
//   - Log validation failures for security monitoring
//   - Use internationalization for global applications
//   - Test validation rules thoroughly with edge cases
//   - Keep validation rules consistent across your application
package validator
