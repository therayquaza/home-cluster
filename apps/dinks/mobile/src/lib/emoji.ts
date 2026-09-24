export const SYMPTOM_EMOJI: Record<string, string> = {
  Cramps: '😣',
  Headache: '🤕',
  Bloating: '🎈',
  Fatigue: '😴',
  'Tender breasts': '💗',
  'Mood swings': '🎭',
  Acne: '🔴',
  Nausea: '🤢',
}

export const MOOD_EMOJI: Record<string, string> = {
  Calm: '😌',
  Happy: '😄',
  Sensitive: '🥺',
  Irritable: '😤',
  Anxious: '😰',
  Sad: '😢',
  Energetic: '⚡',
  Tired: '🥱',
  Confident: '😎',
  Stressed: '😩',
  Grateful: '🥰',
}

export function symptomEmoji(kind: string) {
  return SYMPTOM_EMOJI[kind] ?? ''
}

export function moodEmoji(mood: string) {
  return MOOD_EMOJI[mood] ?? ''
}
