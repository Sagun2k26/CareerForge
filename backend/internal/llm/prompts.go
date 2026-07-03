package llm

import (
	"fmt"
	"strings"
)

// Task markers are appended to each system prompt. Real providers treat them as
// harmless metadata; the mock provider keys off them to return realistic,
// schema-correct fixtures. Centralizing prompts here is the single source of
// truth for everything the system asks a model to do.
const (
	TaskResumeParse  = "resume_parse"
	TaskGapAnalysis  = "gap_analysis"
	TaskRoadmap      = "roadmap"
	TaskInterview    = "interview"
	TaskScoreAnswer  = "score_answer"
	TaskModelAnswer  = "model_answer"
	TaskTailorResume = "tailor_resume"
)

func marker(task string) string { return "\n\n[task:" + task + "]" }

// taskFromSystem extracts the task marker embedded by the builders below.
func taskFromSystem(system string) string {
	i := strings.LastIndex(system, "[task:")
	if i < 0 {
		return ""
	}
	rest := system[i+len("[task:"):]
	if j := strings.IndexByte(rest, ']'); j >= 0 {
		return rest[:j]
	}
	return ""
}

// ---- Resume parsing -------------------------------------------------------

func ResumeParsePrompt(resumeText string) (system, user string) {
	system = `You are an expert technical recruiter. Extract a structured profile from the resume text.
Return ONLY JSON matching this schema:
{
  "name": string,
  "headline": string,
  "skills": [string],
  "experience": [{"title": string, "company": string, "duration": string, "highlights": [string]}],
  "education": [{"degree": string, "institution": string, "year": string}],
  "summary": string
}` + marker(TaskResumeParse)
	user = "Resume text:\n\n" + truncate(resumeText, 12000)
	return
}

// ---- Skill-gap analysis ---------------------------------------------------

func GapAnalysisPrompt(resumeJSON, roleName string, requiredSkills []string) (system, user string) {
	system = `You are a career coach. Compare the candidate profile against the target role's required skills.
Return ONLY JSON matching this schema:
{
  "gap_score": number,            // 0-100, how ready the candidate is
  "matched_skills": [string],
  "missing_skills": [string],
  "summary": string               // 2-3 sentences of honest, encouraging feedback
}` + marker(TaskGapAnalysis)
	user = fmt.Sprintf("Target role: %s\nRequired skills: %s\n\nCandidate profile JSON:\n%s",
		roleName, strings.Join(requiredSkills, ", "), truncate(resumeJSON, 8000))
	return
}

// ---- Roadmap generation ---------------------------------------------------

func RoadmapPrompt(roleName string, missingSkills []string) (system, user string) {
	system = `You are a senior engineer building a focused upskilling plan.
Return ONLY JSON matching this schema:
{
  "items": [
    {
      "order": number,
      "topic": string,
      "why": string,                 // why this matters for the role
      "resources": [string],         // 2-3 concrete resources
      "milestone": string            // how the learner proves mastery
    }
  ]
}
Produce one item per missing skill, ordered from foundational to advanced.` + marker(TaskRoadmap)
	user = fmt.Sprintf("Target role: %s\nSkills to learn: %s", roleName, strings.Join(missingSkills, ", "))
	return
}

// ---- Mock interview -------------------------------------------------------

// InterviewerSystem builds the system prompt for the stateful interviewer. The
// conversation messages are supplied separately by the service.
func InterviewerSystem(roleName string) string {
	return fmt.Sprintf(`You are a friendly but rigorous interviewer for a %s role.
Ask exactly ONE question at a time. Keep questions focused and progressively harder.
Do not score here; just ask the next question or, if the candidate has answered
several questions well, say "INTERVIEW_COMPLETE" on its own line.`, roleName) + marker(TaskInterview)
}

func ScoreAnswerPrompt(roleName, question, answer string) (system, user string) {
	system = `You are an interview evaluator. Score the candidate's answer.
Return ONLY JSON matching this schema:
{
  "clarity": number,        // 0-10
  "correctness": number,    // 0-10
  "depth": number,          // 0-10
  "feedback": string        // 1-2 sentences, specific and actionable
}` + marker(TaskScoreAnswer)
	user = fmt.Sprintf("Role: %s\nQuestion: %s\nCandidate answer: %s", roleName, question, answer)
	return
}

// ---- Question bank --------------------------------------------------------

func ModelAnswerPrompt(roleName, question string) (system, user string) {
	system = `You are a staff engineer writing an exemplary interview answer.
Return ONLY JSON matching this schema:
{
  "model_answer": string,   // a strong, well-structured answer
  "key_points": [string]    // the 3-5 things a great answer must hit
}` + marker(TaskModelAnswer)
	user = fmt.Sprintf("Role: %s\nQuestion: %s", roleName, question)
	return
}

// ---- Resume tailoring -----------------------------------------------------

func TailorResumePrompt(resumeJSON, jobDescription string) (system, user string) {
	system = `You tailor resumes to a specific job description without inventing experience.
Reorder and rewrite bullet points to surface the most relevant achievements and
mirror the job's language. Return ONLY JSON matching this schema:
{
  "summary": string,
  "highlighted_skills": [string],
  "experience": [{"title": string, "company": string, "bullets": [string]}],
  "changes": [string]       // short notes on what you changed and why
}` + marker(TaskTailorResume)
	user = fmt.Sprintf("Job description:\n%s\n\nCandidate profile JSON:\n%s",
		truncate(jobDescription, 6000), truncate(resumeJSON, 8000))
	return
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n...[truncated]"
}
