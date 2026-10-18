import { cn } from "../../lib/utils";
import { motion, useReducedMotion } from "framer-motion";
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useState,
} from "react";

function splitIntoGraphemes(value) {
  if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
    const segmenter = new Intl.Segmenter("en", { granularity: "grapheme" });
    return Array.from(segmenter.segment(value), ({ segment }) => segment);
  }
  return Array.from(value);
}

function getSegments(text, splitBy) {
  let animatedIndex = 0;

  if (splitBy === "lines") {
    return text.split("\n").map((line) => {
      const animated = line.length > 0;
      return {
        value: line,
        animated,
        index: animated ? animatedIndex++ : -1,
      };
    });
  }

  if (splitBy === "characters") {
    return splitIntoGraphemes(text).map((character) => {
      const animated = !/\s/.test(character);
      return {
        value: character,
        animated,
        index: animated ? animatedIndex++ : -1,
      };
    });
  }

  return text.split(/(\s+/).map((part) => {
    const animated = !/^\s+$/.test(part) && part.length > 0;
    return {
      value: part,
      animated,
      index: animated ? animatedIndex++ : -1,
    };
  });
}

function getDelay(index, total, stagger, staggerFrom) {
  if (typeof staggerFrom === "number") {
    return Math.abs(staggerFrom - index) * stagger;
  }

  if (staggerFrom === "end") {
    return (total - 1 - index) * stagger;
  }

  if (staggerFrom === "center") {
    return Math.abs((total - 1) / 2 - index) * stagger;
  }

  if (staggerFrom === "edges") {
    return Math.min(index, total - 1 - index) * stagger;
  }

  if (staggerFrom === "random") {
    const seeded = Math.abs(Math.sin(index * 12.9898) * 43758.5453) % 1;
    return Math.floor(seeded * total) * stagger;
  }

  return index * stagger;
}

function getOffset(direction, distance) {
  if (direction === "down") return { x: 0, y: -distance };
  if (direction === "left") return { x: distance, y: 0 };
  if (direction === "right") return { x: -distance, y: 0 };
  return { x: 0, y: distance };
}

export const KineticTextReveal = forwardRef(
  (
    {
      text,
      className,
      segmentClassName,
      maskClassName,
      splitBy = "words",
      direction = "up",
      distance = 20,
      stagger = 0.075,
      staggerFrom = "start",
      transition = { duration: 0.72, ease: [0.22, 1, 0.36, 1] },
      blur = true,
      autoPlay = true,
      delay = 0,
      onRevealStart,
      onRevealComplete,
      ...props
    },
    ref
  ) => {
    const shouldReduceMotion = useReducedMotion();
    const [run, setRun] = useState(0);
    const [visible, setVisible] = useState(false);

    const segments = useMemo(() => getSegments(text, splitBy), [text, splitBy]);
    const animatedTotal = segments.filter((segment) => segment.animated).length;

    useImperativeHandle(ref, () => ({
      play: () => {
        setVisible(false);
        requestAnimationFrame(() => {
          setRun((current) => current + 1);
          setVisible(true);
          onRevealStart?.();
        });
      },
      reset: () => setVisible(false),
    }));

    useEffect(() => {
      if (!autoPlay) return;

      const timeout = window.setTimeout(() => {
        setRun((current) => current + 1);
        setVisible(true);
        onRevealStart?.();
      }, delay * 1000);

      return () => window.clearTimeout(timeout);
    }, [autoPlay, delay, text, onRevealStart]);

    const offset = getOffset(direction, distance);

    const variants = {
      hidden: shouldReduceMotion
        ? { opacity: 0 }
        : {
            opacity: 0,
            x: offset.x,
            y: offset.y,
            filter: blur ? "blur(6px)" : "blur(0px)",
          },
      visible: (index) => ({
        opacity: 1,
        x: 0,
        y: 0,
        filter: "blur(0px)",
        transition: shouldReduceMotion
          ? { duration: 0.01 }
          : {
              ...transition,
              delay: getDelay(index, animatedTotal, stagger, staggerFrom),
            },
      }),
    };

    return (
      <span
        className={cn(
          "inline-flex flex-wrap whitespace-pre-wrap align-baseline",
          splitBy === "lines" && "flex-col items-start",
          className
        )}
        aria-label={text}
        {...props}
      >
        <span className="sr-only">{text}</span>
        {segments.map((segment, index) => {
          if (!segment.animated) {
            return (
              <span key={`${run}-${index}`} aria-hidden="true">
                {segment.value}
              </span>
            );
          }

          return (
            <span
              key={`${run}-${index}`}
              className={cn(
                "inline-block overflow-hidden align-baseline pb-1",
                maskClassName
              )}
              aria-hidden="true"
            >
              <motion.span
                custom={segment.index}
                variants={variants}
                initial="hidden"
                animate={visible ? "visible" : "hidden"}
                className={cn(
                  "inline-block will-change-transform",
                  segmentClassName
                )}
                onAnimationComplete={
                  segment.index === animatedTotal - 1
                    ? onRevealComplete
                    : undefined
                }
              >
                {segment.value}
              </motion.span>
            </span>
          );
        })}
      </span>
    );
  }
);

KineticTextReveal.displayName = "KineticTextReveal";

