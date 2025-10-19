# Import the Tkinter module for GUI creation
import tkinter as tk

# Create the main application window
root = tk.Tk()
root.title("Thanks Chase!")  # Set the title of the window

# Configure the window size (optional but recommended for better visibility)
root.geometry("400x300")  # Width x Height

# Create a Label widget to display the styled message
message_label = tk.Label(
    root,
    text="Thanks Chase!",
    font=("Arial", 36, "bold"),  # Large, bold text using Arial font
    fg="blue"                   # Set text color to blue for better visibility
)

# Pack the label into the window and center it
message_label.pack(expand=True)  # Expand the label to fill available space

# Start the Tkinter event loop to display the GUI
root.mainloop()
