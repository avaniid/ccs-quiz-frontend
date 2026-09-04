export default function Disqualified() {
  return (
    <div className="min-h-screen flex items-center justify-center p-6">
      <div className="card max-w-3xl w-full p-8 md:p-12 text-center shadow-xs">
        <h1 className="font-display text-2xl md:text-3xl font-bold text-red-600 mb-3">
          You have been disqualified for suspicious activity.
        </h1>
        <p className="text-gray-600 text-base md:text-lg">
          Your test session has been terminated.
        </p>
      </div>
    </div>
  );
}