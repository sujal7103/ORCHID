import { useToastStore } from '@/store/useToastStore';
import { Toast } from './Toast';
import './ToastContainer.css';

export const ToastContainer = () => {
  const { toasts, removeToast } = useToastStore();

  if (toasts.length === 0) {
    return null;
  }

  return (
    <div className="toast-container">
      {toasts.map(t => (
        <Toast
          key={t.id}
          variant={t.type}
          title={t.title}
          message={t.message}
          duration={0} // We handle duration in the store
          onClose={() => removeToast(t.id)}
          action={t.action}
        />
      ))}
    </div>
  );
};
