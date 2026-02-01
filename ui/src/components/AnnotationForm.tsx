import { useState } from 'react';

interface AnnotationFormProps {
  onSubmit: (note: string) => Promise<void>;
}

export default function AnnotationForm({ onSubmit }: AnnotationFormProps) {
  const [note, setNote] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!note.trim() || submitting) return;

    setSubmitting(true);
    try {
      await onSubmit(note.trim());
      setNote('');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="flex gap-2">
      <input
        type="text"
        value={note}
        onChange={(e) => setNote(e.target.value)}
        placeholder="Add a note..."
        className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        disabled={submitting}
      />
      <button
        type="submit"
        disabled={!note.trim() || submitting}
        className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {submitting ? 'Adding...' : 'Add'}
      </button>
    </form>
  );
}
