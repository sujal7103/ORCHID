import { Modal } from '@/components/design-system/feedback/Modal/Modal';
import styles from './ConfirmDialog.module.css';

export interface ConfirmDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  variant?: 'danger' | 'warning' | 'info';
}

export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  isOpen,
  onClose,
  onConfirm,
  title,
  message,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  variant = 'info',
}) => {
  const handleConfirm = () => {
    onConfirm();
    onClose();
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size="sm"
      closeOnBackdrop={true}
      closeOnEscape={true}
      showClose={false}
    >
      <div className={styles.confirmDialog}>
        <p className={styles.message}>{message}</p>
        <div className={styles.actions}>
          <button onClick={onClose} className={styles.cancelButton}>
            {cancelText}
          </button>
          <button onClick={handleConfirm} className={`${styles.confirmButton} ${styles[variant]}`}>
            {confirmText}
          </button>
        </div>
      </div>
    </Modal>
  );
};
