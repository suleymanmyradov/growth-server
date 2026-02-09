package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5434
	user     = "growthmind"
	password = "growthmind123"
	dbname   = "growthmind"
)

type DB struct {
	*sql.DB
}

func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Cannot connect to database:", err)
	}

	fmt.Println("Successfully connected to database!")

	database := &DB{db}

	err = database.SeedData()
	if err != nil {
		log.Fatal("Error seeding data:", err)
	}

	fmt.Println("Data seeded successfully!")
}

func (db *DB) SeedData() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	hashedPassword := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

	userIDs := make([]uuid.UUID, 3)
	userIDs[0] = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userIDs[1] = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	userIDs[2] = uuid.MustParse("00000000-0000-0000-0000-000000000003")

	users := []struct {
		ID           uuid.UUID
		Username     string
		Email        string
		PasswordHash string
		FullName     string
	}{
		{userIDs[0], "john_doe", "john@example.com", string(hashedPassword), "John Doe"},
		{userIDs[1], "jane_smith", "jane@example.com", string(hashedPassword), "Jane Smith"},
		{userIDs[2], "bob_wilson", "bob@example.com", string(hashedPassword), "Bob Wilson"},
	}

	for _, u := range users {
		_, err = tx.Exec(`
			INSERT INTO users (id, username, email, password_hash, full_name, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (username) DO NOTHING`,
			u.ID, u.Username, u.Email, u.PasswordHash, u.FullName, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting user %s: %w", u.Username, err)
		}
	}

	profileIDs := make([]uuid.UUID, 3)
	for i := range userIDs {
		profileIDs[i] = uuid.MustParse(fmt.Sprintf("10000000-0000-0000-0000-%012d", i+1))
	}

	profiles := []struct {
		ID        uuid.UUID
		UserID    uuid.UUID
		Bio       string
		Location  string
		Website   string
		Interests []string
		AvatarURL string
	}{
		{profileIDs[0], userIDs[0], "Passionate about personal growth and productivity", "San Francisco, CA", "https://johndoe.com", []string{"fitness", "productivity", "reading"}, "https://example.com/avatars/john.jpg"},
		{profileIDs[1], userIDs[1], "Yoga enthusiast and mindfulness practitioner", "New York, NY", "https://janesmith.com", []string{"yoga", "meditation", "health"}, "https://example.com/avatars/jane.jpg"},
		{profileIDs[2], userIDs[2], "Tech lover and continuous learner", "Austin, TX", "https://bobwilson.com", []string{"technology", "coding", "learning"}, "https://example.com/avatars/bob.jpg"},
	}

	for _, p := range profiles {
		_, err = tx.Exec(`
			INSERT INTO profiles (id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (user_id) DO NOTHING`,
			p.ID, p.UserID, p.Bio, p.Location, p.Website, p.Interests, p.AvatarURL, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting profile for user %s: %w", p.UserID, err)
		}
	}

	habitIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		habitIDs[i] = uuid.MustParse(fmt.Sprintf("20000000-0000-0000-0000-%012d", i+1))
	}

	habits := []struct {
		ID          uuid.UUID
		Name        string
		Description string
		Streak      int
		Completed   bool
		Category    string
		UserID      uuid.UUID
	}{
		{habitIDs[0], "Morning Meditation", "10 minutes of mindfulness meditation", 30, true, "wellness", userIDs[0]},
		{habitIDs[1], "Daily Reading", "Read at least 20 pages", 15, false, "learning", userIDs[0]},
		{habitIDs[2], "Evening Walk", "30 minute walk after dinner", 21, true, "fitness", userIDs[0]},
		{habitIDs[3], "Morning Yoga", "20 minutes of yoga", 45, true, "wellness", userIDs[1]},
		{habitIDs[4], "Journal Writing", "Write down thoughts and goals", 10, false, "mindfulness", userIDs[1]},
		{habitIDs[5], "Code Practice", "Practice coding for 1 hour", 7, true, "learning", userIDs[2]},
	}

	for _, h := range habits {
		_, err = tx.Exec(`
			INSERT INTO habits (id, name, description, streak, completed, category, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			h.ID, h.Name, h.Description, h.Streak, h.Completed, h.Category, h.UserID, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting habit %s: %w", h.Name, err)
		}
	}

	goalIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		goalIDs[i] = uuid.MustParse(fmt.Sprintf("30000000-0000-0000-0000-%012d", i+1))
	}

	goals := []struct {
		ID          uuid.UUID
		Title       string
		Description string
		Category    string
		DueDate     time.Time
		Progress    int
		Completed   bool
		UserID      uuid.UUID
	}{
		{goalIDs[0], "Complete Marathon Training", "Train for and complete a full marathon", "fitness", time.Now().AddDate(0, 6, 0), 60, false, userIDs[0]},
		{goalIDs[1], "Read 50 Books This Year", "Read 50 books across various genres", "learning", time.Now().AddDate(1, 0, 0), 25, false, userIDs[0]},
		{goalIDs[2], "Learn Spanish", "Achieve conversational level in Spanish", "learning", time.Now().AddDate(0, 9, 0), 40, false, userIDs[0]},
		{goalIDs[3], "Start Yoga Practice", "Establish a daily yoga routine", "wellness", time.Now().AddDate(0, 3, 0), 80, false, userIDs[1]},
		{goalIDs[4], "Meditation Mastery", "Reach 100 consecutive days of meditation", "mindfulness", time.Now().AddDate(0, 4, 0), 45, false, userIDs[1]},
		{goalIDs[5], "Build a Side Project", "Create and launch a web application", "career", time.Now().AddDate(0, 8, 0), 30, false, userIDs[2]},
	}

	for _, g := range goals {
		_, err = tx.Exec(`
			INSERT INTO goals (id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			g.ID, g.Title, g.Description, g.Category, g.DueDate, g.Progress, g.Completed, g.UserID, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting goal %s: %w", g.Title, err)
		}
	}

	goalHabitRelations := []struct {
		GoalID  uuid.UUID
		HabitID uuid.UUID
	}{
		{goalIDs[0], habitIDs[2]},
		{goalIDs[1], habitIDs[1]},
		{goalIDs[3], habitIDs[3]},
		{goalIDs[4], habitIDs[4]},
		{goalIDs[5], habitIDs[5]},
	}

	for _, rel := range goalHabitRelations {
		_, err = tx.Exec(`
			INSERT INTO goal_habit_relations (id, goal_id, habit_id, created_at)
			VALUES ($1, $2, $3, $4)`,
			uuid.New(), rel.GoalID, rel.HabitID, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting goal-habit relation: %w", err)
		}
	}

	articleIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		articleIDs[i] = uuid.MustParse(fmt.Sprintf("40000000-0000-0000-0000-%012d", i+1))
	}

	articles := []struct {
		ID       uuid.UUID
		Title    string
		Excerpt  string
		Content  string
		Category string
		ReadTime int
		ImageURL string
		Author   string
	}{
		{articleIDs[0], "The Science of Habit Formation", "Understanding how habits work and how to build better ones", "Habits are the building blocks of our daily lives. Research shows that 40% of our actions are habits rather than conscious decisions...", "productivity", 8, "https://example.com/images/habits.jpg", "Dr. James Clear"},
		{articleIDs[1], "Mindfulness for Beginners", "A practical guide to starting your meditation journey", "Mindfulness meditation has been shown to reduce stress, improve focus, and enhance overall well-being...", "wellness", 6, "https://example.com/images/mindfulness.jpg", "Sarah Johnson"},
		{articleIDs[2], "The Power of Goal Setting", "How to set and achieve meaningful goals in your life", "Effective goal setting is more than just writing down what you want to achieve. It requires clarity, commitment, and a solid plan...", "productivity", 10, "https://example.com/images/goals.jpg", "Michael Chen"},
		{articleIDs[3], "Building a Reading Habit", "Tips for making reading a daily practice", "Reading is one of the most powerful habits you can develop. It expands your knowledge, improves your vocabulary, and stimulates your mind...", "learning", 7, "https://example.com/images/reading.jpg", "Emily Davis"},
		{articleIDs[4], "The Benefits of Morning Routines", "How starting your day right can transform your life", "Your morning routine sets the tone for the entire day. A well-structured morning can boost productivity and reduce stress...", "productivity", 5, "https://example.com/images/morning.jpg", "Alex Thompson"},
		{articleIDs[5], "Finding Balance in Life", "Strategies for maintaining work-life balance", "In today's fast-paced world, finding balance between work, family, and personal time is more important than ever...", "wellness", 9, "https://example.com/images/balance.jpg", "Rachel Green"},
	}

	for _, a := range articles {
		_, err = tx.Exec(`
			INSERT INTO articles (id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			a.ID, a.Title, a.Excerpt, a.Content, a.Category, a.ReadTime, a.ImageURL, a.Author, time.Now().AddDate(-1, 0, 0), time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting article %s: %w", a.Title, err)
		}
	}

	savedItems := []struct {
		ItemType string
		ItemID   uuid.UUID
		UserID   uuid.UUID
	}{
		{"article", articleIDs[0], userIDs[0]},
		{"article", articleIDs[1], userIDs[0]},
		{"habit", habitIDs[0], userIDs[0]},
		{"goal", goalIDs[0], userIDs[0]},
		{"article", articleIDs[2], userIDs[1]},
		{"habit", habitIDs[3], userIDs[1]},
		{"article", articleIDs[4], userIDs[2]},
		{"goal", goalIDs[5], userIDs[2]},
	}

	for _, item := range savedItems {
		_, err = tx.Exec(`
			INSERT INTO saved_items (id, item_type, item_id, user_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id, item_type, item_id) DO NOTHING`,
			uuid.New(), item.ItemType, item.ItemID, item.UserID, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting saved item: %w", err)
		}
	}

	conversationIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		conversationIDs[i] = uuid.MustParse(fmt.Sprintf("50000000-0000-0000-0000-%012d", i+1))
	}

	conversations := []struct {
		ID          uuid.UUID
		Title       string
		ItemType    string
		LastMessage string
		UserID      uuid.UUID
	}{
		{conversationIDs[0], "Productivity Coaching", "coach", "How can I improve my daily productivity?", userIDs[0]},
		{conversationIDs[1], "Wellness Support", "therapist", "I've been feeling stressed lately", userIDs[1]},
		{conversationIDs[2], "Career Guidance", "coach", "How do I advance in my tech career?", userIDs[2]},
	}

	for _, c := range conversations {
		_, err = tx.Exec(`
			INSERT INTO conversations (id, title, item_type, last_message, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			c.ID, c.Title, c.ItemType, c.LastMessage, c.UserID, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting conversation %s: %w", c.Title, err)
		}
	}

	messageIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		messageIDs[i] = uuid.MustParse(fmt.Sprintf("60000000-0000-0000-0000-%012d", i+1))
	}

	messages := []struct {
		ID             uuid.UUID
		Content        string
		Role           string
		ConversationID uuid.UUID
	}{
		{messageIDs[0], "Hello! I'd like to discuss improving my productivity.", "user", conversationIDs[0]},
		{messageIDs[1], "Great! Productivity is about working smarter, not harder. What specific challenges are you facing?", "assistant", conversationIDs[0]},
		{messageIDs[2], "I struggle with time management and staying focused.", "user", conversationIDs[0]},
		{messageIDs[3], "Hi, I need help managing my stress levels.", "user", conversationIDs[1]},
		{messageIDs[4], "I'm here to help. Can you tell me more about what's causing you stress?", "assistant", conversationIDs[1]},
		{messageIDs[5], "I want to advance in my career as a software developer.", "user", conversationIDs[2]},
	}

	for _, m := range messages {
		_, err = tx.Exec(`
			INSERT INTO messages (id, content, role, conversation_id, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			m.ID, m.Content, m.Role, m.ConversationID, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting message: %w", err)
		}
	}

	notificationIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		notificationIDs[i] = uuid.MustParse(fmt.Sprintf("70000000-0000-0000-0000-%012d", i+1))
	}

	notifications := []struct {
		ID       uuid.UUID
		Title    string
		Message  string
		ItemType string
		Read     bool
		UserID   uuid.UUID
	}{
		{notificationIDs[0], "Habit Reminder", "Time for your morning meditation!", "habit_reminder", false, userIDs[0]},
		{notificationIDs[1], "Goal Deadline", "Your marathon training goal is due in 6 months", "goal_deadline", false, userIDs[0]},
		{notificationIDs[2], "Achievement Unlocked", "You've completed 30 days of meditation!", "achievement", true, userIDs[0]},
		{notificationIDs[3], "Habit Reminder", "Don't forget your evening yoga session", "habit_reminder", false, userIDs[1]},
		{notificationIDs[4], "Goal Deadline", "Your yoga practice goal is due in 3 months", "goal_deadline", false, userIDs[1]},
		{notificationIDs[5], "System Update", "New features available in your dashboard", "system", false, userIDs[2]},
	}

	for _, n := range notifications {
		_, err = tx.Exec(`
			INSERT INTO notifications (id, title, message, item_type, read, user_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			n.ID, n.Title, n.Message, n.ItemType, n.Read, n.UserID, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting notification %s: %w", n.Title, err)
		}
	}

	activityIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		activityIDs[i] = uuid.MustParse(fmt.Sprintf("80000000-0000-0000-0000-%012d", i+1))
	}

	activities := []struct {
		ID          uuid.UUID
		ItemType    string
		Title       string
		Description string
		Metadata    map[string]interface{}
		UserID      uuid.UUID
	}{
		{activityIDs[0], "habit_completed", "Morning Meditation Completed", "Completed 30-day meditation streak", map[string]interface{}{"habit_id": habitIDs[0].String(), "streak": 30}, userIDs[0]},
		{activityIDs[1], "goal_created", "New Goal Created", "Set a goal to complete marathon training", map[string]interface{}{"goal_id": goalIDs[0].String()}, userIDs[0]},
		{activityIDs[2], "article_saved", "Article Saved", "Saved 'The Science of Habit Formation' to read later", map[string]interface{}{"article_id": articleIDs[0].String()}, userIDs[0]},
		{activityIDs[3], "habit_completed", "Morning Yoga Completed", "Completed 45-day yoga practice", map[string]interface{}{"habit_id": habitIDs[3].String(), "streak": 45}, userIDs[1]},
		{activityIDs[4], "goal_created", "New Goal Created", "Set a goal for meditation mastery", map[string]interface{}{"goal_id": goalIDs[4].String()}, userIDs[1]},
		{activityIDs[5], "article_saved", "Article Saved", "Saved 'The Power of Goal Setting' for reference", map[string]interface{}{"article_id": articleIDs[2].String()}, userIDs[2]},
	}

	for _, a := range activities {
		metadataJSON := fmt.Sprintf(`{"habit_id":"%s","streak":30}`, habitIDs[0].String())
		if a.ItemType == "goal_created" {
			metadataJSON = fmt.Sprintf(`{"goal_id":"%s"}`, goalIDs[0].String())
		} else if a.ItemType == "article_saved" {
			metadataJSON = fmt.Sprintf(`{"article_id":"%s"}`, articleIDs[0].String())
		}

		_, err = tx.Exec(`
			INSERT INTO activities (id, item_type, title, description, metadata, user_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			a.ID, a.ItemType, a.Title, a.Description, metadataJSON, a.UserID, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting activity %s: %w", a.Title, err)
		}
	}

	userSettings := []struct {
		UserID             uuid.UUID
		Theme              string
		Language           string
		Timezone           string
		EmailNotifications bool
		PushNotifications  bool
		HabitReminders     bool
		GoalReminders      bool
	}{
		{userIDs[0], "dark", "en", "America/Los_Angeles", true, true, true, true},
		{userIDs[1], "light", "en", "America/New_York", true, false, true, true},
		{userIDs[2], "system", "en", "America/Chicago", false, true, false, true},
	}

	for _, s := range userSettings {
		_, err = tx.Exec(`
			INSERT INTO user_settings (id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, user_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (user_id) DO NOTHING`,
			uuid.New(), s.Theme, s.Language, s.Timezone, s.EmailNotifications, s.PushNotifications, s.HabitReminders, s.GoalReminders, s.UserID, time.Now(), time.Now())
		if err != nil {
			return fmt.Errorf("error inserting user settings for user %s: %w", s.UserID, err)
		}
	}

	articleShareIDs := make([]uuid.UUID, 6)
	for i := 0; i < 6; i++ {
		articleShareIDs[i] = uuid.MustParse(fmt.Sprintf("90000000-0000-0000-0000-%012d", i+1))
	}

	articleShares := []struct {
		ID        uuid.UUID
		ArticleID uuid.UUID
		UserID    uuid.UUID
		Platform  string
	}{
		{articleShareIDs[0], articleIDs[0], userIDs[0], "twitter"},
		{articleShareIDs[1], articleIDs[1], userIDs[0], "facebook"},
		{articleShareIDs[2], articleIDs[2], userIDs[1], "linkedin"},
		{articleShareIDs[3], articleIDs[3], userIDs[1], "twitter"},
		{articleShareIDs[4], articleIDs[4], userIDs[2], "whatsapp"},
		{articleShareIDs[5], articleIDs[5], userIDs[2], "email"},
	}

	for _, share := range articleShares {
		_, err = tx.Exec(`
			INSERT INTO article_shares (id, article_id, user_id, platform, created_at)
			VALUES ($1, $2, $3, $4, $5)`,
			share.ID, share.ArticleID, share.UserID, share.Platform, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting article share: %w", err)
		}
	}

	fmt.Println("Seeded data:")
	fmt.Printf("- %d users\n", len(users))
	fmt.Printf("- %d profiles\n", len(profiles))
	fmt.Printf("- %d habits\n", len(habits))
	fmt.Printf("- %d goals\n", len(goals))
	fmt.Printf("- %d goal-habit relations\n", len(goalHabitRelations))
	fmt.Printf("- %d articles\n", len(articles))
	fmt.Printf("- %d saved items\n", len(savedItems))
	fmt.Printf("- %d conversations\n", len(conversations))
	fmt.Printf("- %d messages\n", len(messages))
	fmt.Printf("- %d notifications\n", len(notifications))
	fmt.Printf("- %d activities\n", len(activities))
	fmt.Printf("- %d user settings\n", len(userSettings))
	fmt.Printf("- %d article shares\n", len(articleShares))

	return nil
}
