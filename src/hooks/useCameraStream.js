import { useEffect, useRef, useState } from "react";
import { acquireCameraStream } from "./cameraStore";

export default function useCameraStream(onViolation) {
  const onViolationRef = useRef(onViolation);
  onViolationRef.current = onViolation;

  const videoRef = useRef(null);
  const streamRef = useRef(null);
  const [stream, setStream] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let cancelled = false;

    const handleTrackEnded = () => {
      onViolationRef.current?.("camera-lost");
    };

    async function start() {
      try {
        // Reuses the stream the instructions page already opened, instead of
        // closing the device and racing to reopen it.
        const mediaStream = await acquireCameraStream();
        if (cancelled) return;
        streamRef.current = mediaStream;
        mediaStream.getTracks().forEach((track) => {
          track.addEventListener("ended", handleTrackEnded);
        });
        setStream(mediaStream);
      } catch (err) {
        if (!cancelled) {
          setError(err);
        }
      }
    }

    start();

    return () => {
      cancelled = true;
      const mediaStream = streamRef.current;
      if (mediaStream) {
        mediaStream.getTracks().forEach((track) => {
          track.removeEventListener("ended", handleTrackEnded);
        });
        streamRef.current = null;
      }
      // Deliberately NOT releasing the device here. React remounts this effect
      // (StrictMode in development, and any re-render of the route), and
      // stopping the camera only to reopen it a moment later is exactly the
      // race that broke proctoring. The stream is released once the attempt
      // ends — see submitQuiz in Quiz.jsx — and the browser reclaims it when
      // the tab closes.
    };
  }, []);

  // Quiz.jsx mounts the <video> only once the paper has loaded, which is
  // usually well after the stream resolved. Keying this on [stream] alone ran
  // it while the element did not exist yet and never again, so the video (and
  // with it face/object detection and the snapshot) stayed blank. Check on
  // every render instead; the identity check keeps it a no-op once bound.
  useEffect(() => {
    const video = videoRef.current;
    if (video && stream && video.srcObject !== stream) {
      video.srcObject = stream;
    }
  });

  return { stream, videoRef, error };
}
