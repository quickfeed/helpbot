package helpbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Raytar/helpbot/models"
	"github.com/bufbuild/connect-go"
	"github.com/bwmarrin/discordgo"
	qfpb "github.com/quickfeed/quickfeed/qf"
)

type command func(m *discordgo.InteractionCreate)

type commandMap map[string]command

func (bot *HelpBot) initCommands() {
	bot.commands = commandMap{
		// base commands
		"help":      bot.helpCommand,
		"register":  bot.registerCommand,
		"configure": bot.configureCommand,

		// student commands
		"gethelp": bot.hasRole(func(m *discordgo.InteractionCreate) { bot.helpRequestCommand(m, "help") }, RoleStudent),
		"approve": bot.hasRole(func(m *discordgo.InteractionCreate) { bot.helpRequestCommand(m, "approve") }, RoleStudent),
		"cancel":  bot.hasRole(bot.cancelRequestCommand, RoleStudent),
		"status":  bot.hasRole(bot.studentStatusCommand, RoleStudent),

		// assistant commands
		"length":         bot.hasRole(bot.lengthCommand, RoleAssistant),
		"list":           bot.hasRole(bot.listCommand, RoleAssistant),
		"next":           bot.hasRole(bot.nextRequestCommand, RoleAssistant),
		"clear":          bot.hasRole(bot.clearCommand, RoleAssistant),
		"unregister":     bot.hasRole(bot.unregisterCommand, RoleAssistant),
		"cancel-waiting": bot.hasRole(bot.assistantCancelCommand, RoleAssistant),
	}
}

var baseHelp = createModal("Available commands",
	``+"```"+`
help:                       Shows this help text
register [course] [GitHub username]: Register your discord account as a student.
`+"```", true)

var studentHelp = createModal("Student Commands",
	``+"```"+`
help:    Shows this help text
gethelp: Request help from a teaching assistant
approve: Get your lab approved by a teaching assistant
cancel:  Cancels your help request and removes you from the queue
status:  Show your position in the queue
`+"```"+`
After requesting help, you can check the response message you got to see your position in the queue.
You will receive a message when you are next in queue.
`, true)

var assistant = createModal("Teaching Assistant Commands",
	``+"```"+`
help:               Shows this help text
length:             Returns the number of students waiting for help.
list <num>:         Lists the next <num> students in the queue.
next:               Removes and returns the first student from the queue.
clear:              Clears the queue!
unregister @mention Unregisters the mentioned user.
cancel              Cancels your 'waiting' status.
`+"```", true)

func (bot *HelpBot) helpCommand(m *discordgo.InteractionCreate) {
	// check if the user has the teaching assistant role
	if bot.hasRoles(m.GuildID, m.Member, RoleAssistant) {
		replyModal(bot.client, m, assistant)
		return
	}
	// check if the user has the student role
	if bot.hasRoles(m.GuildID, m.Member, RoleStudent) {
		replyModal(bot.client, m, studentHelp)
		return
	}
	// user has no roles, show base help
	replyModal(bot.client, m, baseHelp)
}

func (bot *HelpBot) helpRequestCommand(m *discordgo.InteractionCreate, requestType string) {
	req := models.HelpRequest{
		StudentUserID: m.Member.User.ID,
		GuildID:       m.GuildID,
		Type:          requestType,
		Done:          false,
	}

	err := bot.db.CreateHelpRequest(&req)
	if err != nil {
		bot.log.Errorln("helpRequest: failed to create new request:", err)
		replyMsg(bot.client, m, fmt.Sprintf("An error occurred while creating your request: %s", err.Error()))
		return
	}

	pos, err := bot.db.GetQueuePosition(m.GuildID, m.Member.User.ID)
	if err != nil {
		bot.log.Errorln("helpRequest: failed to get pos in queue after creating request:", err)
		replyMsg(bot.client, m, "An error occurred while creating your request.")
		return
	}

	replyMsg(bot.client, m, fmt.Sprintf("A help request has been created, and you are at position %d in the queue.", pos))
}

func (bot *HelpBot) studentStatusCommand(m *discordgo.InteractionCreate) {
	pos, err := bot.db.GetQueuePosition(m.GuildID, m.Member.User.ID)
	if err != nil {
		bot.log.Errorln("studentStatus: failed to get position in queue:", err)
		replyMsg(bot.client, m, "An error occurred.")
		return
	}
	if pos <= 0 {
		replyMsg(bot.client, m, "You are not in the queue.")
		return
	}
	replyMsg(bot.client, m, fmt.Sprintf("You are at position %d in the queue.", pos))
}

func (bot *HelpBot) cancelRequestCommand(m *discordgo.InteractionCreate) {
	if err := bot.db.CancelHelpRequest(m.GuildID, m.Member.User.ID); err != nil {
		replyMsg(bot.client, m, fmt.Sprintf("No active request found: %s", err))
	} else {
		replyMsg(bot.client, m, "Your request was cancelled.")
	}
}

func (bot *HelpBot) nextRequestCommand(m *discordgo.InteractionCreate) {
	request, err := bot.db.AssignNextRequest(m.Member.User.ID, m.GuildID)
	if err != nil {
		bot.log.Errorf("Failed to assign next request: %v by user: %s in guild: %s", err, m.Member.User.ID, m.GuildID)
		replyMsg(bot.client, m, fmt.Sprintf("Failed to assign next request: %s", err))
		return
	}

	if request == nil || request.StudentUserID == "" {
		replyMsg(bot.client, m, "No requests in queue.")
		return
	}
	student, err := bot.client.GuildMember(m.GuildID, request.StudentUserID)
	if err != nil {
		bot.log.Errorln("Failed to fetch user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	if !replyMsg(bot.client, m, fmt.Sprintf("Next '%s' request is by %s.", request.Type, getMentionAndNick(student))) {
		return
	}
	sendMsg(bot.client, student.User, fmt.Sprintf("You will now receive help from %s", getMentionAndNick(m.Member)))
}

func (bot *HelpBot) lengthCommand(m *discordgo.InteractionCreate) {
	requests, err := bot.db.GetWaitingRequests(m.GuildID, 0)
	if err != nil {
		replyMsg(bot.client, m, "An error occurred.")
		return
	}
	var msg string
	if len(requests) == 1 {
		msg = "There is 1 student waiting for help."
	} else {
		msg = fmt.Sprintf("There are %d students waiting for help.", len(requests))
	}
	replyMsg(bot.client, m, msg)
}

func (bot *HelpBot) listCommand(m *discordgo.InteractionCreate) {
	num := 10

	options := m.ApplicationCommandData().Options

	var err error

	if len(options) >= 1 {
		num = int(options[0].IntValue())
	}

	var sb strings.Builder
	requests, err := bot.db.GetWaitingRequests(m.GuildID, num)
	if err != nil {
		replyMsg(bot.client, m, "Failed to get list of requests.")
		return
	}

	if len(requests) == 0 {
		replyMsg(bot.client, m, "There are no open requests.")
		return
	}

	fmt.Fprintf(&sb, "Showing the next %d requests:\n\n", len(requests))
	for i, req := range requests {
		user, err := bot.client.GuildMember(m.GuildID, req.StudentUserID)
		if err != nil {
			bot.log.Errorln("Failed to obtain user info:", err)
			replyMsg(bot.client, m, "An error occurred while sending the message")
			return
		}
		fmt.Fprintf(&sb, "%d. User: %s, Type: %s\n", i+1, getMentionAndNick(user), req.Type)
	}
	replyMsg(bot.client, m, sb.String())
}

func (bot *HelpBot) clearCommand(m *discordgo.InteractionCreate) {
	data := m.ApplicationCommandData().Options
	if len(data) < 1 || data[0].Type != discordgo.ApplicationCommandOptionBoolean {
		replyMsg(bot.client, m, "You must specify whether to clear the queue or not.")
		return
	}

	if !data[0].BoolValue() {
		replyMsg(bot.client, m, "No changes were made to the queue.")
		return
	}

	// TODO: send a message to each student whose request was cleared.
	if err := bot.db.ClearHelpRequests(m.Member.User.ID, m.GuildID); err != nil {
		bot.log.Errorln("Failed to clear queue:", err)
		replyMsg(bot.client, m, "Clear failed due to an error.")
		return
	}

	replyMsg(bot.client, m, "The queue was cleared.")
}

func (bot *HelpBot) registerCommand(m *discordgo.InteractionCreate) {
	// Check if course is configured
	course, err := bot.db.GetCourse(&models.Course{GuildID: m.GuildID})
	if err != nil {
		bot.log.Errorln("Failed to get course:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	// Check if role exists
	studentRole := bot.GetRole(m.GuildID, RoleStudent)
	if len(studentRole) == 0 {
		fmt.Printf("Failed to find student role for register command. Message: %+v Data: %+v\n", m.Message, m.Data)
		replyMsg(bot.client, m, "Failed to find student role.")
		return
	}

	if len(m.ApplicationCommandData().Options) == 0 {
		fmt.Printf("No github login provided for register command. Message: %+v Data: %+v\n", m.Message, m.Data)
		replyMsg(bot.client, m, "You must include your github username in the command.")
		return
	}
	githubLogin, ok := m.ApplicationCommandData().Options[0].Value.(string)
	if !ok {
		fmt.Printf("Failed to parse github login for register command. Message: %+v Data: %+v\n", m.Message, m.Data)
		replyMsg(bot.client, m, "You must include your github username in the command.")
		return
	}

	student, err := bot.db.GetStudent(&models.Student{UserID: m.Member.User.ID, GithubLogin: githubLogin})
	if err != nil {
		bot.log.Errorln("Failed to check for existing user:", err)
		replyMsg(bot.client, m, "An unknown error occurred.")
		return
	}

	if student != nil {
		replyMsg(bot.client, m, "That github login has already been used to register! If you believe that this is a mistake, please contact a teaching assistant.")
		return
	}

	newStudent := models.Student{
		UserID:      m.Member.User.ID,
		GithubLogin: githubLogin,
	}

	// TODO: query autograder for real name, using github login for now
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req := &qfpb.EnrollmentRequest{
		FetchMode: &qfpb.EnrollmentRequest_CourseID{CourseID: uint64(course.CourseID)},
	}
	enrollments, err := bot.qf.qf.GetEnrollments(ctx, connect.NewRequest(req))
	if err != nil {
		bot.log.Errorln("Failed to get info from QuickFeed:", err)
		replyMsg(bot.client, m, "Failed to communicate with QuickFeed")
		return
	}

	var enrollment *qfpb.Enrollment
	for _, e := range enrollments.Msg.GetEnrollments() {
		if e.GetUser().GetLogin() == githubLogin {
			enrollment = e
			break
		}
	}

	if enrollment.GetUser() == nil {
		replyMsg(bot.client, m, "Failed to find your enrollment in the course")
		return
	}

	newStudent.Name = enrollment.GetUser().GetName()
	newStudent.UserID = m.Member.User.ID
	newStudent.GithubLogin = githubLogin
	newStudent.GuildID = m.GuildID

	switch enrollment.GetStatus() {
	case qfpb.Enrollment_STUDENT:
		if err := bot.db.CreateStudent(&newStudent); err != nil {
			replyMsg(bot.client, m, "An uknown error occurred.")
			return
		}
		if err := bot.client.GuildMemberRoleAdd(m.GuildID, newStudent.UserID, studentRole); err != nil {
			bot.log.Errorln("Failed to add student role:", err)
			replyMsg(bot.client, m, "Failed to give you the student role.")
			return
		}
	case qfpb.Enrollment_TEACHER:
		if _, err := bot.db.GetOrCreateAssistant(&models.Assistant{
			UserID:  newStudent.UserID,
			GuildID: m.GuildID,
		}); err != nil {
			bot.log.Errorln("Failed to create assistant:", err)
			replyMsg(bot.client, m, "Failed to create assistant.")
		}
		if err := bot.client.GuildMemberRoleAdd(m.GuildID, newStudent.UserID, bot.GetRole(m.GuildID, RoleAssistant)); err != nil {
			bot.log.Errorln("Failed to add assistant role:", err)
			replyMsg(bot.client, m, "Failed to give you the assistant role.")
			return
		}
	default: // pending or none (not enrolled)
		bot.log.Errorf("User is not enrolled in the course: (%s, %s)", newStudent.UserID, newStudent.GithubLogin)
		replyMsg(bot.client, m, "You are not enrolled in the course.")
		return
	}

	if err := bot.client.GuildMemberNickname(m.GuildID, newStudent.UserID, newStudent.Name); err != nil {
		bot.log.Errorln("Failed to set nick:", err)
		replyMsg(bot.client, m, "Failed to set your nickname.")
		return
	}

	replyMsg(bot.client, m, fmt.Sprintf(
		"Authentication was successful! You should now have more access to the server. Type %shelp to see available commands",
		bot.cfg.Prefix))
}

func (bot *HelpBot) unregisterCommand(m *discordgo.InteractionCreate) {
	if m.Member == nil {
		replyMsg(bot.client, m, "You must be a member of the server to use this command.")
		return
	}

	user := m.Member.User

	// permanent deletion from db
	err := bot.db.DeleteStudent(&models.Student{UserID: user.ID})
	if err != nil {
		replyMsg(bot.client, m, "Failed to delete user info.")
		bot.log.Errorln("Failed to delete student info:", err)
		return
	}

	// remove nickname
	if err := bot.client.GuildMemberNickname(m.GuildID, user.ID, ""); err != nil {
		replyMsg(bot.client, m, "Failed to remove user nick. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to remove user nick:", err)
		return
	}

	// unassign role
	if err := bot.client.GuildMemberRoleRemove(m.GuildID, user.ID, bot.cfg.StudentRole); err != nil {
		replyMsg(bot.client, m, "Failed to remove user roles. You may have to remove role/nickname manually.")
		bot.log.Errorln("Failed to remove user roles:", err)
		return
	}
	replyMsg(bot.client, m, "User was unregistered.")
}

func (bot *HelpBot) assistantCancelCommand(m *discordgo.InteractionCreate) {
	err := bot.db.CancelWaitingAssistant(m.Member.User.ID)
	if err != nil {
		replyMsg(bot.client, m, fmt.Sprintf("Failed to cancel waiting status: %v", err))
		return
	}
	replyMsg(bot.client, m, fmt.Sprintf("Your waiting status was removed (you will have to use %snext again to get the next student)", bot.cfg.Prefix))
}

// hasRole returns a function that checks if the user has the specified role, and then calls the original function.
// If the user does not have the role, a message is sent to the user saying that they do not have permission to use the command.
// This function is used to wrap the command functions to check if the user has the required role.
func (bot *HelpBot) hasRole(f func(*discordgo.InteractionCreate), roles ...string) func(*discordgo.InteractionCreate) {
	return func(m *discordgo.InteractionCreate) {
		if !bot.hasRoles(m.GuildID, m.Member, roles...) {
			replyMsg(bot.client, m, "You do not have permission to use this command.")
			return
		}
		f(m)
	}
}

func (bot *HelpBot) configureCommand(m *discordgo.InteractionCreate) {
	if m.Data.Type() != discordgo.InteractionApplicationCommand {
		bot.log.Errorln("Received non-application command interaction")
		return
	}

	data := m.ApplicationCommandData()

	if len(data.Options) == 0 || data.Options[0].Value == nil {
		replyMsg(bot.client, m, "Please provide a course name")
		return
	}
	course, err := bot.db.GetCourse(&models.Course{Name: data.Options[0].Value.(string)})
	if err != nil {
		replyMsg(bot.client, m, fmt.Sprintf("Failed to get course: %v", err))
		return
	}

	if course.GuildID != "" {
		replyMsg(bot.client, m, "This course is already configured for another server.")
		return
	}

	course.GuildID = m.GuildID
	if err := bot.db.UpdateCourse(course); err != nil {
		replyMsg(bot.client, m, fmt.Sprintf("Failed to update course: %v", err))
		return
	}

	if err = bot.initServer(bot.client, m.GuildID); err != nil {
		replyMsg(bot.client, m, fmt.Sprintf("Failed to configure server: %v", err))
		return
	}
	replyMsg(bot.client, m, fmt.Sprintf("Server was configured for course %s", course.Name))
}
