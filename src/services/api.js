export async function fetchQuestions(quizId) {
    const { mockQuestions } = await import("../mock/mockQuestions");
    return mockQuestions;
}

export async function submitQuiz(answers, flagsRaised) {
    console.log("Mock submit:", answers, flagsRaised);
    return { status: "ok" };
}