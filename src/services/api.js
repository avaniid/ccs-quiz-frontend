export async function fetchQuestions(quizId) {
    const { mockQuestions } = await import("../mock/mockQuestions");
    return mockQuestions;
}

export async function submitQuiz(answers, flagsRaised, snapshotImage) {
    const payload = {
        Responses: answers,
        FlagsRaised: flagsRaised,
        Image: snapshotImage || null,
    };
    console.log("Mock submit:", payload);
    return { status: "ok" };
}