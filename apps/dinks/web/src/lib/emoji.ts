export const SYMPTOM_EMOJI: Record<string, string> = {
  Cramps: '😣',
  Headache: '🤕',
  Bloating: '🎈',
  Fatigue: '😴',
  'Tender breasts': '💗',
  'Mood swings': '🎭',
  Acne: '🔴',
  Nausea: '🤢',
  'Abdominal pain': '😖',
  'Increased appetite': '🍽️',
  Backache: '🦴',
  'Vaginal itching': '🌡️',
  Insomnia: '🌙',
  Diarrhea: '💧',
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
  Confused: '🌫️',
}

export function symptomEmoji(kind: string) {
  return SYMPTOM_EMOJI[kind] ?? ''
}

export function moodEmoji(mood: string) {
  return MOOD_EMOJI[mood] ?? ''
}
